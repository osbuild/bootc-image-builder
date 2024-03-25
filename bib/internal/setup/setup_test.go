package setup_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/bootc-image-builder/bib/internal/setup"
)

var (
	procSelfMountinfoBtrfs = `1345 1307 0:92 / / rw,relatime - overlay overlay rw,context="system_u:object_r:container_file_t:s0:c98,c684",lowerdir=/var/lib/containers/storage/overlay/l/T4G5ED56VRUEC2ID7JMJSGW3QT:/var/lib/containers/storage/overlay/l/KZGLWVLLUSCKZYWV57JF6WSK2I:/var/lib/containers/storage/overlay/l/HXWMTP6IYXVK4KUZ2UCSNJXOIW:/var/lib/containers/storage/overlay/l/YLBHA4OHH25PNVUIUV7LKJA67Q:/var/lib/containers/storage/overlay/l/7IH6AODKJYPJSHYY3KTKRAGDZO:/var/lib/containers/storage/overlay/l/EELW6SDH3FRQMJJEGMTWJ223DL:/var/lib/containers/storage/overlay/l/GNKPUG6OGVJANN4MB447IM4JK7,upperdir=/var/lib/containers/storage/overlay/f2d065994b6e5d208732fc71a7421f882a6b2f73809bcc210a00d4c49309c10f/diff,workdir=/var/lib/containers/storage/overlay/f2d065994b6e5d208732fc71a7421f882a6b2f73809bcc210a00d4c49309c10f/work,redirect_dir=on,uuid=on,metacopy=on,volatile
1346 1345 0:96 / /sys rw,nosuid,nodev,noexec,relatime - sysfs sysfs rw,seclabel
1348 1345 0:97 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw
1349 1345 0:99 / /dev rw,nosuid - tmpfs tmpfs rw,context="system_u:object_r:container_file_t:s0:c98,c684",size=65536k,mode=755,inode64
1352 1349 0:100 / /dev/pts rw,nosuid,noexec,relatime - devpts devpts rw,context="system_u:object_r:container_file_t:s0:c98,c684",gid=5,mode=620,ptmxmode=666
1361 1349 0:85 / /dev/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,context="system_u:object_r:container_file_t:s0:c98,c684",size=64000k,inode64
1363 1346 0:27 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime - cgroup2 cgroup2 rw,seclabel,nsdelegate,memory_recursiveprot
1364 1345 0:33 /root/var/lib/containers/storage /var/lib/containers/storage rw,relatime - btrfs /dev/mapper/luks-12345678-1234-1234-1234-123456789012 rw,seclabel,compress=zstd:1,ssd,discard=async,space_cache,subvolid=257,subvol=/root
1365 1364 0:33 /root/var/lib/containers/storage/overlay /var/lib/containers/storage/overlay rw,relatime - btrfs /dev/mapper/luks-12345678-1234-1234-1234-123456789012 rw,seclabel,compress=zstd:1,ssd,discard=async,space_cache,subvolid=257,subvol=/root
`
	procSelfMountinfoWithVolume = `735 665 0:83 / / rw,relatime - overlay overlay rw,lowerdir=/var/lib/containers/storage/overlay/l/2BFF257G6Q5T5OWGW4JVUDTM3R:/var/lib/containers/storage/overlay/l/AEC37A7HVU4SH5Q4GWQLB7KCZU:/var/lib/containers/storage/overlay/l/JFOUQY5ZWN3DIZQP22SYYLXEV7,upperdir=/var/lib/containers/storage/overlay/d545c8ca670b6820eae79e43836175558d71f9f927fb68feffa608a2b3020f09/diff,workdir=/var/lib/containers/storage/overlay/d545c8ca670b6820eae79e43836175558d71f9f927fb68feffa608a2b3020f09/work,uuid=on,volatile
736 735 259:2 /var/lib/containers/storage/volumes/e14ef96e3db8bdc3dcdbd340208995e6770cd7873f78788e737c8b7b5d17fdaa/_data /output rw,nosuid,nodev,relatime - ext4 /dev/nvme0n1p2 rw,errors=remount-ro
737 735 0:86 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw
738 735 0:87 / /dev rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755,inode64
741 735 0:88 / /sys rw,nosuid,nodev,noexec,relatime - sysfs sysfs rw
742 738 0:89 / /dev/pts rw,nosuid,noexec,relatime - devpts devpts rw,gid=5,mode=620,ptmxmode=666
743 738 0:85 / /dev/mqueue rw,nosuid,nodev,noexec,relatime - mqueue mqueue rw
744 735 0:24 /containers/storage/overlay-containers/5e5914d9351994648b28b8b58ba3c67ce06a48ccdd3a52679b82ae4dc693fa53/userdata/.containerenv /run/.containerenv rw,relatime - tmpfs tmpfs rw,size=3262508k,mode=755,inode64
745 735 0:24 /containers/storage/overlay-containers/5e5914d9351994648b28b8b58ba3c67ce06a48ccdd3a52679b82ae4dc693fa53/userdata/hostname /etc/hostname rw,relatime - tmpfs tmpfs rw,size=3262508k,mode=755,inode64
746 735 0:24 /containers/storage/overlay-containers/5e5914d9351994648b28b8b58ba3c67ce06a48ccdd3a52679b82ae4dc693fa53/userdata/resolv.conf /etc/resolv.conf rw,relatime - tmpfs tmpfs rw,size=3262508k,mode=755,inode64
747 735 0:24 /containers/storage/overlay-containers/5e5914d9351994648b28b8b58ba3c67ce06a48ccdd3a52679b82ae4dc693fa53/userdata/hosts /etc/hosts rw,relatime - tmpfs tmpfs rw,size=3262508k,mode=755,inode64
748 738 0:82 / /dev/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=64000k,inode64
749 741 0:28 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime - cgroup2 cgroup2 rw,nsdelegate,memory_recursiveprot
750 735 259:2 /var/lib/containers/storage /var/lib/containers/storage rw,relatime - ext4 /dev/nvme0n1p2 rw,errors=remount-ro
751 750 259:2 /var/lib/containers/storage/overlay /var/lib/containers/storage/overlay rw,relatime - ext4 /dev/nvme0n1p2 rw,errors=remount-ro
`

	procSelfMountinfoNoVolume = `736 735 259:2 /var/lib/containers/storage/volumes/5c45612492556520d9549b3ee24d6a11ac3a0e9266c0900bf427a3bdaafadeb8/_data /rpmmd rw,nosuid,nodev,relatime - ext4 /dev/nvme0n1p2 rw,errors=remount-ro
737 735 259:2 /var/lib/containers/storage/volumes/e17849f054757adebabc844a1fd510308a1c22b6668526e84511a8dfd617aec8/_data /output rw,nosuid,nodev,relatime - ext4 /dev/nvme0n1p2 rw,errors=remount-ro
738 735 0:86 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw
739 735 259:2 /var/lib/containers/storage/volumes/8b15ab68602eaee4923cfddd7c0cbb19d7d33190c9d2f97adeea357370f5ba3b/_data /store rw,nosuid,nodev,relatime - ext4 /dev/nvme0n1p2 rw,errors=remount-ro
740 735 0:87 / /sys rw,nosuid,nodev,noexec,relatime - sysfs sysfs rw
741 735 0:88 / /dev rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755,inode64
742 741 0:85 / /dev/mqueue rw,nosuid,nodev,noexec,relatime - mqueue mqueue rw
743 741 0:89 / /dev/pts rw,nosuid,noexec,relatime - devpts devpts rw,gid=5,mode=620,ptmxmode=666
745 741 0:82 / /dev/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=64000k,inode64
750 735 259:2 /var/lib/containers/storage/volumes/8612bf74115e4610cf4a57da8a0209fa07d1782584752bf4a1991d773f9c608f/_data /var/lib/containers/storage rw,nosuid,nodev,relatime - ext4 /dev/nvme0n1p2 rw,errors=remount-ro
666 741 0:89 /0 /dev/console rw,relatime - devpts devpts rw,gid=5,mode=620,ptmxmode=666
`
)

func TestValidateHasContainerStorageMountedBtrfs(t *testing.T) {
	restore := setup.MockInsideContainer(func() (bool, error) {
		return true, nil
	})
	defer restore()

	for _, tc := range []struct {
		fakeMountinfo string
		expectedErr   string
	}{
		// good
		{procSelfMountinfoBtrfs, ""},
		{procSelfMountinfoWithVolume, ""},
		// bad
		{procSelfMountinfoNoVolume, `cannot find suffix "/var/lib/containers/storage" in mounted "/var/lib/containers/storage/volumes/8612bf74115e4610cf4a57da8a0209fa07d1782584752bf4a1991d773f9c608f/_data"`},
	} {
		buf := bytes.NewBufferString(tc.fakeMountinfo)
		err := setup.ValidateHasContainerStorageMountedFromReader(buf)
		if tc.expectedErr == "" {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
			assert.Equal(t, tc.expectedErr, err.Error())
		}
	}
}
