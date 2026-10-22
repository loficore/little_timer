// Command little-timer-migrate-keystore —— 一次性凭据迁移 CLI（issue #13）。
//
// 把 v1 主口令时代的凭据搬进平台 keystore：
//
//   - settings 的 backup_config 旧密文列（webdav_password / s3_access_key /
//     s3_secret_key，免口令）；
//   - secret.db 的 v1 密文（LTMSECv1 || salt || nonce || sealed，需 v1 主口令）。
//
// 正常情况下应用启动时会自动跑同一条迁移路径（见 cmd/server 的启动序列），
// 本工具用于两种场景：
//
//  1. 用户想先在命令行确认能读懂旧数据（--dry-run，不写任何东西）；
//  2. 无桌面环境（如 headless 服务器）需要离线完成迁移。
//
// 幂等：数据目录下的 .keystore-migrated 哨兵存在即跳过，重复执行无副作用。
//
// 退出码：
//
//	0 —— 迁移成功 / 已完成 / dry-run 校验通过。
//	1 —— 参数错误、数据库打不开、迁移失败（含主口令错误）。
//	2 —— keystore 不可用（无 Secret Service / 无 StrongBox 等环境问题）。
//
// 用法：
//
//	$ little-timer-migrate-keystore --db-path /var/lib/little_timer.db
//	$ little-timer-migrate-keystore --db-path ./db/little_timer.db --dry-run
//	$ LITTLE_TIMER_V1_PASSWORD=... little-timer-migrate-keystore --db-path ...
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"little-timer/internal/keystore"
	"little-timer/internal/log"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
)

// passwordEnv 是 v1 主口令的环境变量名。命令行 --password 会进 argv
// （同机其他用户可从 /proc/<pid>/cmdline 读到），所以优先用环境变量。
const passwordEnv = "LITTLE_TIMER_V1_PASSWORD"

// 退出码。
const (
	exitOK         = 0
	exitFailure    = 1
	exitNoKeystore = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run 抽出真实逻辑，便于测试直接调用而不必拦截 os.Exit。
func run(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("migrate-keystore", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbPath := fs.String("db-path", "", "little_timer SQLite 路径（必填）")
	dir := fs.String("dir", "", "数据目录（默认取 DB 同级；secret.db 位于此）")
	password := fs.String("password", "", "v1 主口令（也可用环境变量 "+passwordEnv+"）")
	dryRun := fs.Bool("dry-run", false, "只校验能否解密，不产生任何写")
	if err := fs.Parse(args); err != nil {
		return exitFailure
	}

	if *dbPath == "" {
		fmt.Fprintln(stderr, "缺少 --db-path")
		fs.Usage()
		return exitFailure
	}
	dataDir := *dir
	if dataDir == "" {
		dataDir = filepath.Dir(*dbPath)
	}

	pwd := []byte(*password)
	if len(pwd) == 0 {
		pwd = []byte(os.Getenv(passwordEnv))
	}

	rep, err := migrate(dataDir, *dbPath, pwd, *dryRun)
	if err != nil {
		fmt.Fprintf(stderr, "迁移失败: %v\n", err)
		if errors.Is(err, keystore.ErrUnavailable) {
			fmt.Fprintln(stderr, "提示: 系统密钥服务不可用。桌面环境通常自带；"+
				"sway / 最小安装可运行 `gnome-keyring-daemon --daemonize --components=secrets`"+
				" 或安装 KWallet / KeePassXC。")
			return exitNoKeystore
		}
		if errors.Is(err, keystore.ErrUnseal) {
			fmt.Fprintln(stderr, "提示: 主口令错误，或 secret 文件已损坏。迁移未做任何改动。")
		}
		return exitFailure
	}

	switch {
	case rep.Skipped:
		fmt.Fprintln(stdout, "已迁移过（哨兵存在），未做任何改动。")
	case rep.DryRun:
		fmt.Printf("dry-run 校验通过：可解密 %d 条凭据、%d 条 secret。未做任何写。\n",
			rep.CredsMoved, rep.SecretsMoved)
	default:
		fmt.Printf("迁移完成：凭据 %d 条、secret %d 条。\n", rep.CredsMoved, rep.SecretsMoved)
		if rep.V1BackedUp != "" {
			fmt.Printf("旧 secret 文件已改名备份至 %s，确认无误后可自行删除。\n", rep.V1BackedUp)
		}
	}
	return exitOK
}

// migrate 打开数据库、构造适配器并跑一次 MigrateAll。
func migrate(dataDir, dbPath string, password []byte, dryRun bool) (keystore.Report, error) {
	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		return keystore.Report{}, fmt.Errorf("打开数据库失败: %w", err)
	}
	defer func() { _ = sqlite.Close() }()
	// 迁移本身不改 schema，但旧库可能还没跑过迁移阶梯；先补齐，
	// 否则读旧密文列会因列不存在而失败。
	if err := sqlite.Migrate(); err != nil {
		return keystore.Report{}, fmt.Errorf("数据库 schema 迁移失败: %w", err)
	}

	sm, err := settings.NewFromSqliteManager(sqlite, dbPath)
	if err != nil {
		return keystore.Report{}, fmt.Errorf("打开 settings 失败: %w", err)
	}

	ks := keystore.Current()
	if err := ks.Available(); err != nil {
		return keystore.Report{}, fmt.Errorf("%w: %v", keystore.ErrUnavailable, err)
	}

	return keystore.MigrateAll(ks, keystore.MigrateOptions{
		// 旧凭据密钥按 DB 路径派生，必须与当初加密时一致。
		DBPath:       dbPath,
		SecretPath:   filepath.Join(dataDir, "secret.db"),
		SentinelPath: filepath.Join(dataDir, keystore.SentinelName),
		Password:     password,
		Creds:        sm,
		DryRun:       dryRun,
	})
}

var _ = log.Info // 保留日志依赖：迁移路径经 keystore 包记录结构化日志。