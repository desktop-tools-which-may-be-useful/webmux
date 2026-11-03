//go:build !unix

package update

import "os"

// fileID has no portable device and inode here; size and modification time
// still tell a replaced executable apart.
func fileID(os.FileInfo) (dev, ino uint64) { return 0, 0 }
