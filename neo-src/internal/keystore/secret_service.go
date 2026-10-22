//go:build linux && !android

// secret_service.go —— XDG Secret Service 的 D-Bus 协议细节(godbus/v5 手写客户端,
// 无 cgo)。Keystore 接口实现见 keystore_linux.go,包文档见 keystore.go。
//
// 会话加密算法说明:设计文档(以及 issue #13 的旧 spec 草稿)要求协商
// `org.freedesktop.secrets.api.encryption.DH-aes256-cbc`。该算法名只存在于
// 2012 年前后的草稿中,实现生态从未采纳:gnome-keyring 3.12/3.28/40 的
// OpenSession 仅接受 "plain" 与 "dh-ietf1024-sha256-aes128-cbc-pkcs7"
// (daemon/dbus/gkd-secret-session.c handle_open;本机 40.0 对草稿算法名直接返回
// NotSupported),libsecret 0.10~0.20、python secretstorage ≥3.1、KWallet ksecretd
// 同样只用后者。本实现采用生态事实标准的 DH+AES128-CBC-PKCS7 会话算法以保证与
// 真实 daemon 互操作;协商失败仍按设计文档 §4 报 ErrUnavailable。
//
// Prompt/Unlock 对话框协议明确不在本期范围(设计文档 §14):daemon 返回 Prompt
// 对象(集合锁定、新建密钥环需口令)时一律 ErrUnavailable。
//
// 取回的 master key 只存在于调用栈内存中,包内无任何日志输出,key 不落日志。

package keystore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	crand "crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/godbus/dbus/v5"
)

const (
	secretBusName = "org.freedesktop.secrets"
	secretObjPath = "/org/freedesktop/secrets"

	serviceIface    = "org.freedesktop.Secret.Service"
	collectionIface = "org.freedesktop.Secret.Collection"
	itemIface       = "org.freedesktop.Secret.Item"
	sessionIface    = "org.freedesktop.Secret.Session"

	// sessionAlgorithm —— 见文件头说明:草稿要求的 DH-aes256-cbc 无实现生态,
	// 采用 gnome-keyring/libsecret/KWallet/KeePassXC 事实标准算法。
	sessionAlgorithm = "dh-ietf1024-sha256-aes128-cbc-pkcs7"

	// itemLabel / contentType —— master key item 的逐字标签与内容类型。
	itemLabel   = "little_timer master_key"
	contentType = "application/octet-stream"

	// collectionLabel —— 默认集合不存在时 CreateCollection 的标签。
	collectionLabel = "little_timer"

	collectionLabelProp = "org.freedesktop.Secret.Collection.Label"
	// itemLabelProp / itemAttributesProp —— CreateItem 属性键按规范须带完整
	// 接口前缀(gnome-keyring 40 的 gkd-secret-property.c 以
	// g_str_has_prefix(prop_name, interface) 解析,裸 "Label" 会被判为
	// "Invalid properties argument")。
	itemLabelProp      = "org.freedesktop.Secret.Item.Label"
	itemAttributesProp = "org.freedesktop.Secret.Item.Attributes"

	// collectionLockedProp —— 集合的 Locked 属性(规范 Collection.Locked)。
	// 为 true 时 SearchItems 不返回其中的 item,写入则直接失败。
	collectionLockedProp = "org.freedesktop.Secret.Collection.Locked"

	// defaultAlias —— ReadAlias 查询默认集合的别名;gnome-keyring 仅支持
	// "default"/"" 两种 CreateCollection alias("Only the 'default' alias is
	// supported"),故建集合时不指定 little_timer 别名。
	defaultAlias = "default"

	// dhPrimeHex —— IETF IKE 1024-bit MODP 群(RFC 2409 §2 group 2,生成元 2),
	// 与 gnome-keyring 的 "ietf-ike-grp-modp-1024" 一致(256 hex = 128 字节)。
	dhPrimeHex = "ffffffffffffffffc90fdaa22168c234c4c6628b80dc1cd1" +
		"29024e088a67cc74020bbea63b139b22514a08798e3404ddef9519b3" +
		"cd3a431b302b0a6df25f14374fe1356d6d51c245e485b576625e7ec6" +
		"f44c42e9a637ed6b0bff5cb6f406b7edee386bfb5a899fa5ae9f2411" +
		"7c4b1fe649286651ece65381ffffffffffffffff"
)

// itemAttrs —— master key item 的 D-Bus 匹配属性(测试逐字断言)。
var itemAttrs = map[string]string{"service": "application:little_timer", "key": "master_key"}

// dhPrime 是 sessionAlgorithm 使用的有限域群模数。
var dhPrime = mustParseDHPrime()

func mustParseDHPrime() *big.Int {
	p, ok := new(big.Int).SetString(dhPrimeHex, 16)
	if !ok {
		panic(fmt.Errorf("keystore: 解析 dhPrimeHex 失败(构建期常量错误)"))
	}
	return p
}

// keyringGuidance —— 设计文档 §9 的指引文案(逐字保留,供 fail-fast 错误透出)。
const keyringGuidance = "需要系统密钥服务(org.freedesktop.secrets)。" +
	"桌面环境通常自带;sway/最小安装可运行 " +
	"`gnome-keyring-daemon --daemonize --components=secrets` " +
	"或安装 KWallet / KeePassXC。"

// unavailable —— 统一把失败包装为 ErrUnavailable,原因在前、§9 指引在后。
func unavailable(reason string) error {
	return fmt.Errorf("%w: %s; %s", ErrUnavailable, reason, keyringGuidance)
}

// secretService 是一次性的 D-Bus 客户端句柄(每个 Keystore 方法新建)。
type secretService struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

// dialSecretService 连接会话总线并确认 org.freedesktop.secrets 可达
// (names 未列出时尝试 D-Bus 激活)。
func dialSecretService() (*secretService, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, unavailable("无法连接会话 D-Bus: " + err.Error())
	}
	if err := ensureServiceReachable(conn); err != nil {
		return nil, err
	}
	return &secretService{conn: conn, obj: conn.Object(secretBusName, secretObjPath)}, nil
}

func ensureServiceReachable(conn *dbus.Conn) error {
	var names []string
	call := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0)
	if err := call.Store(&names); err != nil {
		return unavailable("ListNames 失败: " + err.Error())
	}
	for _, n := range names {
		if n == secretBusName {
			return nil
		}
	}
	// 尚未运行:请求 bus 激活(service 文件等价于
	// gnome-keyring-daemon --start --components=secrets)。
	act := conn.BusObject().Call("org.freedesktop.DBus.StartServiceByName", 0,
		secretBusName, uint32(0))
	if act.Err != nil {
		return unavailable("会话总线上没有密钥服务,且激活失败: " + act.Err.Error())
	}
	return nil
}

// isEmptyPath 报告 D-Bus 空对象("/"、"."、"")。
func isEmptyPath(p dbus.ObjectPath) bool { return p == "" || p == "/" || p == "." }

// dhSession 是一次已协商的加密会话(16 字节 AES-128 会话密钥)。
type dhSession struct {
	path   dbus.ObjectPath
	aesKey []byte
}

// secretStruct —— Secret Service 的 Secret 结构 "(oayays)":会话路径、
// AES-CBC IV、会话加密后的值、内容类型。字段顺序即 D-Bus struct 顺序。
type secretStruct struct {
	Session dbus.ObjectPath
	IV      []byte
	Value   []byte
	Type    string
}

// openSession 执行 DH 协商:本地指数为 128 字节随机数,公钥 2^x mod p 以
// 128 字节大端 ay 传输;共享密钥 = 对端公钥^x mod p,经 HKDF-SHA256
// (全零 salt、空 info, RFC 5869)派生 16 字节 AES-128 会话密钥。
func (s *secretService) openSession() (*dhSession, error) {
	expBytes := make([]byte, 128)
	if _, err := crand.Read(expBytes); err != nil {
		return nil, unavailable("生成 DH 私钥失败: " + err.Error())
	}
	exp := new(big.Int).SetBytes(expBytes)
	myPublic := bePad(new(big.Int).Exp(big.NewInt(2), exp, dhPrime), 128)

	peerPub, sessionPath, err := s.exchangePublicValue(myPublic)
	if err != nil {
		return nil, err
	}
	shared := new(big.Int).Exp(peerPub, exp, dhPrime)
	return &dhSession{
		path:   sessionPath,
		aesKey: hkdfSHA256(bePad(shared, 128), 16),
	}, nil
}

// exchangePublicValue 调用 OpenSession,返回对端 DH 公钥(ay)与会话对象路径。
func (s *secretService) exchangePublicValue(myPublic []byte) (*big.Int, dbus.ObjectPath, error) {
	call := s.obj.Call(serviceIface+".OpenSession", 0, sessionAlgorithm, dbus.MakeVariant(myPublic))
	if call.Err != nil {
		return nil, "", unavailable(fmt.Sprintf("OpenSession(%s) 失败: %v", sessionAlgorithm, call.Err))
	}
	var out dbus.Variant
	var path dbus.ObjectPath
	if err := call.Store(&out, &path); err != nil {
		return nil, "", unavailable("OpenSession 响应解析失败: " + err.Error())
	}
	peerBytes, ok := out.Value().([]byte)
	if !ok {
		return nil, "", unavailable("OpenSession 响应不含字节型对端公钥")
	}
	if isEmptyPath(path) {
		return nil, "", unavailable("OpenSession 未返回会话路径")
	}
	return new(big.Int).SetBytes(peerBytes), path, nil
}

// close 尽力而为地关闭会话;失败忽略(daemon 会在连接释放时回收)。
func (s *secretService) close(sess *dhSession) {
	_ = s.conn.Object(secretBusName, sess.path).Call(sessionIface+".Close", 0).Store()
}

// encrypt 把明文打包成会话加密后的 Secret 结构(随机 IV + PKCS#7 + AES-CBC)。
func (sess *dhSession) encrypt(plaintext []byte) (secretStruct, error) {
	iv := make([]byte, aes.BlockSize)
	if _, err := crand.Read(iv); err != nil {
		return secretStruct{}, unavailable("生成 CBC IV 失败: " + err.Error())
	}
	block, err := aes.NewCipher(sess.aesKey)
	if err != nil {
		return secretStruct{}, unavailable("会话密钥无效: " + err.Error())
	}
	padded := pkcs7Pad(plaintext, aes.BlockSize)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)
	return secretStruct{Session: sess.path, IV: iv, Value: out, Type: contentType}, nil
}

// decrypt 解开会话加密的 Secret 结构;失败按通道错误(ErrUnavailable)报。
func (sess *dhSession) decrypt(sec secretStruct) ([]byte, error) {
	if len(sess.aesKey) != 16 {
		return nil, unavailable("会话密钥长度异常")
	}
	block, err := aes.NewCipher(sess.aesKey)
	if err != nil {
		return nil, unavailable("会话密钥无效: " + err.Error())
	}
	if len(sec.IV) != aes.BlockSize || len(sec.Value) == 0 || len(sec.Value)%aes.BlockSize != 0 {
		return nil, unavailable("Secret 结构的 IV/密文布局非法")
	}
	out := make([]byte, len(sec.Value))
	cipher.NewCBCDecrypter(block, sec.IV).CryptBlocks(out, sec.Value)
	plain, err := pkcs7Strip(out, aes.BlockSize)
	if err != nil {
		return nil, unavailable("会话解密(PKCS#7)失败: " + err.Error())
	}
	return plain, nil
}

// searchMasterKey 用 itemAttrs 检索 master key item。
// 返回值:未锁定 item 路径、是否存在(含锁定)、是否被锁定。
func (s *secretService) searchMasterKey() (item dbus.ObjectPath, found, locked bool, err error) {
	var unlocked, lockedPaths []dbus.ObjectPath
	call := s.obj.Call(serviceIface+".SearchItems", 0, map[string]string(itemAttrs))
	if call.Err != nil {
		return "", false, false, unavailable("SearchItems 失败: " + call.Err.Error())
	}
	if err := call.Store(&unlocked, &lockedPaths); err != nil {
		return "", false, false, unavailable("SearchItems 响应解析失败: " + err.Error())
	}
	if len(unlocked) > 0 {
		return unlocked[0], true, false, nil
	}
	if len(lockedPaths) > 0 {
		return "", true, true, nil
	}
	return "", false, false, nil
}

// defaultCollection 返回默认集合路径;不存在则 CreateCollection(标签 little_timer)。
// 若 daemon 返回 Prompt(新建密钥环需要交互口令),按 §14 报 ErrUnavailable。
func (s *secretService) defaultCollection() (dbus.ObjectPath, error) {
	var path dbus.ObjectPath
	call := s.obj.Call(serviceIface+".ReadAlias", 0, defaultAlias)
	if call.Err != nil {
		return "", unavailable("ReadAlias(default) 失败: " + call.Err.Error())
	}
	if err := call.Store(&path); err != nil {
		return "", unavailable("ReadAlias 响应解析失败: " + err.Error())
	}
	if !isEmptyPath(path) {
		// 别名已存在是**正常路径**（任何真实桌面、以及 CI 上都是如此），所以
		// Locked 检查必须落在这一支上。早先的实现在这里直接 return，导致
		// usableCollection 只挂在下面的 CreateCollection 分支后 —— 那条分支
		// 几乎永不执行，整个锁定态处理等于死代码。
		return s.usableCollection(path)
	}
	props := map[string]dbus.Variant{
		collectionLabelProp: dbus.MakeVariant(collectionLabel),
	}
	create := s.obj.Call(serviceIface+".CreateCollection", 0, props, "")
	if create.Err != nil {
		return "", unavailable("CreateCollection 失败: " + create.Err.Error())
	}
	var coll, prompt dbus.ObjectPath
	if err := create.Store(&coll, &prompt); err != nil {
		return "", unavailable("CreateCollection 响应解析失败: " + err.Error())
	}
	if !isEmptyPath(prompt) {
		return "", unavailable("默认密钥环不存在,新建需要交互式口令(Prompt 协议本期不实现);" +
			"请先登录桌面或以 gnome-keyring-daemon --unlock --daemonize 创建并解锁密钥环")
	}
	if isEmptyPath(coll) {
		return "", unavailable("CreateCollection 未返回集合路径")
	}
	return s.usableCollection(coll)
}

// usableCollection 校验集合是否可写,必要时按规范解锁后返回。
//
// 为什么必须有这一步:集合被锁定时,SearchItems **不返回**其中的 item,
// 于是 searchMasterKey 返回 found=false,EnsureMasterKey 会误判成
// "密钥环里没有 master key",转而往这个锁定集合里 CreateItem —— 报出来的
// 是 "master key 所在集合已锁定",既晚(拖到第一次 Seal)又让人误以为是
// 用户数据问题。锁定态必须在这里就地识别。
//
// 解锁走规范 org.freedesktop.Secret.Service.Unlock(不在 Prompt 路径时
// daemon 会直接解锁;gnome-keyring 对有口令的 login 集合会返回 Prompt,
// 那种情况无交互无法解锁,如实报错让运维去解锁登录密钥环)。
func (s *secretService) usableCollection(path dbus.ObjectPath) (dbus.ObjectPath, error) {
	locked, err := s.collectionLocked(path)
	if err != nil {
		return "", err
	}
	if !locked {
		return path, nil
	}
	unlocked, prompt, err := s.unlock(path)
	if err != nil {
		return "", err
	}
	if !isEmptyPath(prompt) {
		return "", unavailable("默认密钥环(" + string(path) +
			")已锁定且需要交互口令才能解锁(CI/无桌面环境下无解);" +
			"请登录桌面会话解锁登录密钥环,或在测试环境改用注入的测试 keystore")
	}
	if len(unlocked) == 0 {
		return "", unavailable("集合解锁未生效:" + string(path))
	}
	return path, nil
}

// collectionLocked 读集合的 Locked 属性。
func (s *secretService) collectionLocked(path dbus.ObjectPath) (bool, error) {
	obj := s.conn.Object(secretBusName, path)
	v, err := obj.GetProperty(collectionLockedProp)
	if err != nil {
		return false, unavailable("读取集合 Locked 属性失败(" + string(path) + "): " + err.Error())
	}
	locked, ok := v.Value().(bool)
	if !ok {
		return false, unavailable("集合 Locked 属性类型异常(" + string(path) + ")")
	}
	return locked, nil
}

// unlock 按规范解锁集合,返回 (已解锁的集合列表, 需交互的 Prompt 对象)。
//
// 注意 prompt 的类型:规范 org.freedesktop.Secret.Service.Unlock 的出参是
// (ao unlocked, o prompt) —— prompt 是**单个 object**,不是数组。
// 早先这里把两者都声明成 []dbus.ObjectPath,导致 daemon 返回单个 o 时
// dbus.Store 报 "cannot convert dbus.ObjectPath to []dbus.ObjectPath",
// 锁定态处理整条路径失效且不报错。该缺陷此前不可见,是因为集合从未被
// 锁定过,Unlock 分支从未执行。
func (s *secretService) unlock(path dbus.ObjectPath) (unlocked []dbus.ObjectPath, prompt dbus.ObjectPath, err error) {
	call := s.obj.Call(serviceIface+".Unlock", 0, []dbus.ObjectPath{path})
	if call.Err != nil {
		return nil, "", unavailable("Unlock 调用失败: " + call.Err.Error())
	}
	if err := call.Store(&unlocked, &prompt); err != nil {
		return nil, "", unavailable("Unlock 响应解析失败: " + err.Error())
	}
	return unlocked, prompt, nil
}

// probeCollectionUsable 供 Available 使用:只判断"能不能往密钥环里存东西",
// 不创建任何 item —— 探测必须是只读的,否则每次启动探测都会留垃圾。
func (s *secretService) probeCollectionUsable() error {
	path, err := s.defaultCollection()
	if err != nil {
		return err
	}
	_ = path
	return nil
}
// storeMasterKey 把 32 字节 master key 作为 item 存入默认集合。
func (s *secretService) storeMasterKey(key []byte) error {
	sess, err := s.openSession()
	if err != nil {
		return err
	}
	defer s.close(sess)

	coll, err := s.defaultCollection()
	if err != nil {
		return err
	}
	sec, err := sess.encrypt(key)
	if err != nil {
		return err
	}
	props := map[string]dbus.Variant{
		itemLabelProp:      dbus.MakeVariant(itemLabel),
		itemAttributesProp: dbus.MakeVariant(map[string]string(itemAttrs)),
	}
	call := s.conn.Object(secretBusName, coll).
		Call(collectionIface+".CreateItem", 0, props, sec, false)
	if call.Err != nil {
		return unavailable("Item.Create 失败: " + call.Err.Error())
	}
	var item, prompt dbus.ObjectPath
	if err := call.Store(&item, &prompt); err != nil {
		return unavailable("Item.Create 响应解析失败: " + err.Error())
	}
	if !isEmptyPath(prompt) {
		return unavailable("创建 item 需要交互式 Prompt(集合可能已锁定),请先解锁登录密钥环")
	}
	return nil
}

// fetchMasterKey 取回并解密 master key;不存在时返回包装 ErrNotFound 的错误。
// 取回的 key 仅驻留调用栈内存,不落日志、不写盘。
func (s *secretService) fetchMasterKey() ([]byte, error) {
	sess, err := s.openSession()
	if err != nil {
		return nil, err
	}
	defer s.close(sess)

	item, found, locked, err := s.searchMasterKey()
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("%w: 密钥环中尚无 master key item", ErrNotFound)
	}
	if locked {
		return nil, unavailable("master key 所在集合已锁定,请先解锁登录密钥环")
	}
	var sec secretStruct
	call := s.conn.Object(secretBusName, item).Call(itemIface+".GetSecret", 0, sess.path)
	if call.Err != nil {
		return nil, unavailable("Item.GetSecret 失败: " + call.Err.Error())
	}
	if err := call.Store(&sec); err != nil {
		return nil, unavailable("Item.GetSecret 响应解析失败: " + err.Error())
	}
	return sess.decrypt(sec)
}

// deleteMasterKey 删除 master key item;不存在时幂等返回 nil。
func (s *secretService) deleteMasterKey() error {
	item, found, locked, err := s.searchMasterKey()
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if locked {
		return unavailable("master key 所在集合已锁定,请先解锁登录密钥环")
	}
	call := s.conn.Object(secretBusName, item).Call(itemIface+".Delete", 0)
	if call.Err != nil {
		return unavailable("Item.Delete 失败: " + call.Err.Error())
	}
	var prompt dbus.ObjectPath
	if err := call.Store(&prompt); err != nil {
		return unavailable("Item.Delete 响应解析失败: " + err.Error())
	}
	if !isEmptyPath(prompt) {
		return unavailable("删除 item 需要交互式 Prompt(集合可能已锁定),请先解锁登录密钥环")
	}
	return nil
}

// bePad 把正整数编码为 size 字节大端(左侧补零)。
func bePad(n *big.Int, size int) []byte {
	b := n.Bytes()
	out := make([]byte, size)
	copy(out[size-len(b):], b)
	return out
}

// hkdfSHA256 —— RFC 5869 HKDF(全零 salt、空 info):
// PRK = HMAC-SHA256(zeros, ikm);T1 = HMAC-SHA256(PRK, 0x01);取前 length 字节。
func hkdfSHA256(ikm []byte, length int) []byte {
	salt := make([]byte, sha256.Size)
	mac := hmac.New(sha256.New, salt)
	mac.Write(ikm)
	prk := mac.Sum(nil)

	mac = hmac.New(sha256.New, prk)
	mac.Write([]byte{0x01})
	t1 := mac.Sum(nil)
	return t1[:length]
}

// pkcs7Pad —— PKCS#7 填充(长度恰为块倍数时补满一整块)。
func pkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - len(data)%blockSize
	out := make([]byte, 0, len(data)+pad)
	out = append(out, data...)
	for i := 0; i < pad; i++ {
		out = append(out, byte(pad))
	}
	return out
}

// pkcs7Strip 去除并校验 PKCS#7 填充。
func pkcs7Strip(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("keystore: 密文长度非法(%d)", len(data))
	}
	pad := int(data[len(data)-1])
	if pad < 1 || pad > blockSize || pad > len(data) {
		return nil, fmt.Errorf("keystore: PKCS#7 填充字节非法(%d)", pad)
	}
	for _, b := range data[len(data)-pad:] {
		if int(b) != pad {
			return nil, fmt.Errorf("keystore: PKCS#7 填充内容不一致")
		}
	}
	return data[:len(data)-pad], nil
}
