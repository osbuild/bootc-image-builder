package pathpolicy

// MountpointPolicies is a set of default mountpoint policies used for filesystem customizations
var MountpointPolicies = NewPathPolicies(map[string]PathPolicy{
	"/": {},
	// /etc must be on the root filesystem
	"/etc": {Deny: true},
	// NB: any mountpoints under /usr are not supported by systemd fstab
	// generator in initram before the switch-root, so we don't allow them.
	"/usr": {Exact: true},
	// API filesystems
	"/sys":  {Deny: true},
	"/proc": {Deny: true},
	"/dev":  {Deny: true},
	"/run":  {Deny: true},
	// not allowed due to merged-usr
	"/bin":   {Deny: true},
	"/sbin":  {Deny: true},
	"/lib":   {Deny: true},
	"/lib64": {Deny: true},
	// used by ext filesystems
	"/lost+found": {Deny: true},
	// used by EFI
	"/boot/efi": {Deny: true},
	// used by systemd / ostree
	"/sysroot": {Deny: true},
	// symlink to ../run which is on tmpfs
	"/var/run": {Deny: true},
	// symlink to ../run/lock which is on tmpfs
	"/var/lock": {Deny: true},
})

// CustomDirectoriesPolicies is a set of default policies for custom directories
var CustomDirectoriesPolicies = NewPathPolicies(map[string]PathPolicy{
	"/":    {Deny: true},
	"/etc": {},
})

// CustomFilesPolicies is a set of default policies for custom files
var CustomFilesPolicies = NewPathPolicies(map[string]PathPolicy{
	"/":           {Deny: true},
	"/etc":        {},
	"/root":       {},
	"/etc/fstab":  {Deny: true},
	"/etc/shadow": {Deny: true},
	"/etc/passwd": {Deny: true},
	"/etc/group":  {Deny: true},
})
