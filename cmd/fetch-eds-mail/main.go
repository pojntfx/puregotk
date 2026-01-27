package main

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/jwijenbergh/puregotk/pkg/core"
	"github.com/jwijenbergh/puregotk/v4/camel"
	"github.com/jwijenbergh/puregotk/v4/edataserver"
	"github.com/jwijenbergh/puregotk/v4/glib"
	"github.com/jwijenbergh/puregotk/v4/gobject"
)

// ptrArrayToStrings converts a GPtrArray (returned as uintptr) to a Go string slice
func ptrArrayToStrings(ptrArrayPtr uintptr) []string {
	if ptrArrayPtr == 0 {
		return nil
	}

	ptrArray := (*glib.PtrArray)(unsafe.Pointer(ptrArrayPtr))
	if ptrArray.Len == 0 {
		return nil
	}

	result := make([]string, ptrArray.Len)
	for i := uint32(0); i < ptrArray.Len; i++ {
		strPtr := *(*uintptr)(unsafe.Pointer(ptrArray.Pdata + uintptr(i)*unsafe.Sizeof(uintptr(0))))
		if strPtr != 0 {
			result[i] = core.GoString(strPtr)
		}
	}

	return result
}

var unsupportedBackends = map[string]bool{
	"none":    true,
	"vfolder": true,
	"rss":     true,
}

var remoteBackends = map[string]bool{
	"imapx": true,
	"imap":  true,
	"pop":   true,
}


func main() {
	camel.Init(glib.GetUserDataDir(), false)

	registry, err := edataserver.NewSourceRegistrySync(nil)
	if err != nil {
		panic(err)
	}

	credentialsProvider := edataserver.NewSourceCredentialsProvider(registry)

	// Create a CamelSession using GObject instantiation with properties
	userDataDir := glib.GetUserDataDir()
	userCacheDir := glib.GetUserCacheDir()

	var userDataDirVal, userCacheDirVal gobject.Value
	userDataDirVal.Init(gobject.TypeStringVal)
	userDataDirVal.SetString(userDataDir)
	userCacheDirVal.Init(gobject.TypeStringVal)
	userCacheDirVal.SetString(userCacheDir)

	sessionObj := gobject.NewObjectWithProperties(
		camel.SessionGLibType(),
		2,
		[]string{"user-data-dir", "user-cache-dir"},
		[]gobject.Value{userDataDirVal, userCacheDirVal},
	)
	session := camel.SessionNewFromInternalPtr(sessionObj.GoPointer())

	sources := registry.ListSources(edataserver.SOURCE_EXTENSION_MAIL_ACCOUNT)

	count := 0
	for s := sources; s != nil; s = s.Next {
		count++
	}
	fmt.Printf("Found %d mail account(s):\n\n", count)

	for s := sources; s != nil; s = s.Next {
		source := edataserver.SourceNewFromInternalPtr(s.Data)
		displayName := source.GetDisplayName()
		uid := source.GetUid()

		fmt.Printf("Mail Account: %s (%s)\n", displayName, uid)

		ext := source.GetExtension(edataserver.SOURCE_EXTENSION_MAIL_ACCOUNT)
		mailAccount := edataserver.SourceMailAccountNewFromInternalPtr(ext.GoPointer())
		backendName := mailAccount.DupBackendName()

		fmt.Printf("  Backend: %s\n", backendName)

		if backendName == "" || unsupportedBackends[backendName] {
			fmt.Printf("  Skipping unsupported backend\n\n")
			continue
		}

		service, err := session.AddService(uid, backendName, camel.ProviderStoreValue)
		if err != nil || service == nil {
			fmt.Printf("  Error: Could not create service: %v\n\n", err)
			continue
		}

		store := camel.StoreNewFromInternalPtr(service.GoPointer())
		source.CamelConfigureService(service)

		isRemote := remoteBackends[backendName]

		if isRemote {
			credSource := credentialsProvider.RefCredentialsSource(source)
			if credSource == nil {
				credSource = source
			}

			var creds *edataserver.NamedParameters
			ok, _ := credentialsProvider.LookupSync(credSource, nil, &creds)
			if ok && creds != nil {
				password := creds.Get(edataserver.SOURCE_CREDENTIAL_PASSWORD)
				if password != "" {
					service.SetPassword(password)
					fmt.Printf("  Credentials found\n")
				}
			}
		}

		connected, err := service.ConnectSync(nil)
		if err != nil {
			if isRemote {
				fmt.Printf("  Note: Could not connect (%v)\n", err)
			} else {
				fmt.Printf("  Error: Could not connect: %v\n\n", err)
				continue
			}
		}

		if !connected && !isRemote {
			fmt.Printf("  Error: Could not connect to store\n\n")
			continue
		}

		folderInfo, err := store.GetFolderInfoSync("", camel.StoreFolderInfoRecursiveValue, nil)
		if err != nil {
			fmt.Printf("  Could not list folders: %v\n", err)
			if isRemote {
				fmt.Printf("  (Remote account may require authentication)\n\n")
			}
			service.DisconnectSync(true, nil)
			continue
		}

		if folderInfo == nil {
			fmt.Printf("  No folders found\n\n")
			service.DisconnectSync(true, nil)
			continue
		}

		processFolders(store, folderInfo, "  ")
		service.DisconnectSync(true, nil)
		fmt.Println()
	}
}

func processFolders(store *camel.Store, info *camel.FolderInfo, indent string) {
	for info != nil {
		displayName := core.GoString(info.DisplayName)
		fullName := core.GoString(info.FullName)
		total := info.Total

		fmt.Printf("%sFolder: %s (%d messages)\n", indent, displayName, total)

		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("%s  Error reading folder contents: %v\n", indent, r)
				}
			}()

			folder, err := store.GetFolderSync(fullName, 0, nil)
			if err != nil {
				fmt.Printf("%s  Error opening folder: %v\n", indent, err)
				return
			}
			if folder == nil {
				return
			}

			folder.RefreshInfoSync(nil)
			uids := ptrArrayToStrings(folder.DupUids())

			if len(uids) > 0 {
				limit := 20
				if len(uids) < limit {
					limit = len(uids)
				}
				fmt.Printf("%s  Showing %d of %d messages:\n\n", indent, limit, len(uids))

				for i := 0; i < limit; i++ {
					msgInfo := folder.GetMessageInfo(uids[i])
					if msgInfo != nil {
						subject := msgInfo.GetSubject()
						if subject == "" {
							subject = "(No subject)"
						}
						fmt.Printf("%s    - Subject: %s\n", indent, subject)

						if from := msgInfo.GetFrom(); from != "" {
							fmt.Printf("%s        From: %s\n", indent, from)
						}
						if to := msgInfo.GetPropertyTo(); to != "" {
							fmt.Printf("%s        To: %s\n", indent, to)
						}
						if dateSent := msgInfo.GetDateSent(); dateSent > 0 {
							t := time.Unix(dateSent, 0)
							fmt.Printf("%s        Date: %s\n", indent, t.Format(time.RFC3339))
						}
						fmt.Println()
					}
				}
			}
		}()

		if info.Child != 0 {
			childInfo := (*camel.FolderInfo)(unsafe.Pointer(info.Child))
			processFolders(store, childInfo, indent+"  ")
		}

		if info.Next != 0 {
			info = (*camel.FolderInfo)(unsafe.Pointer(info.Next))
		} else {
			break
		}
	}
}
