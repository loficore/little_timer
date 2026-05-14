/* Minimal shim for windows headers so @cImport won't fail on non-Windows hosts.
   When compiling on Windows the real system headers are used via _WIN32.
*/
#ifndef WINCRED_SHIM_H
#define WINCRED_SHIM_H

#if defined(_WIN32) || defined(_WIN64)
#include <windows.h>
#include <wincred.h>
#else
typedef unsigned short WORD;
typedef unsigned long DWORD;
typedef unsigned long ULONG;
typedef unsigned short WCHAR;
typedef const WCHAR *LPCWSTR;
typedef WCHAR *LPWSTR;

typedef struct _FILETIME {
    DWORD dwLowDateTime;
    DWORD dwHighDateTime;
} FILETIME;

/* Minimal CREDENTIAL struct with the fields we use from wincred.h */
typedef struct _CREDENTIALW {
    LPWSTR TargetName;
    DWORD Type;
    LPWSTR Comment;
    FILETIME LastWritten;
    DWORD CredentialBlobSize;
    unsigned char *CredentialBlob;
    DWORD Persist;
    DWORD AttributeCount;
    void *Attributes;
    LPWSTR TargetAlias;
    LPWSTR UserName;
} CREDENTIALW, *PCREDENTIALW;

/* Provide names expected by the Zig code: Credential and Persist */
typedef struct Credential {
    LPWSTR target_name;
    DWORD type;
    LPWSTR comment;
    FILETIME last_written;
    DWORD credential_blob_size;
    unsigned char *credential_blob;
    DWORD persist;
    DWORD attribute_count;
    void *attributes;
    LPWSTR target_alias;
    LPWSTR user_name;
} Credential;

typedef enum Persist {
    Session = 1,
    LocalMachine = 2,
    Enterprise = 3,
} Persist;

typedef enum _CRED_PERSIST {
    CRED_PERSIST_SESSION = 1,
    CRED_PERSIST_LOCAL_MACHINE = 2,
    CRED_PERSIST_ENTERPRISE = 3
} CRED_PERSIST;

#endif /* _WIN32 */

#endif /* WINCRED_SHIM_H */
