package arch

import (
	"runtime"
)

type Arch uint64

const ( // architecture enum
	ARCH_AARCH64 Arch = iota
	ARCH_PPC64LE
	ARCH_S390X
	ARCH_X86_64
)

func (a Arch) String() string {
	switch a {
	case ARCH_AARCH64:
		return "aarch64"
	case ARCH_PPC64LE:
		return "ppc64le"
	case ARCH_S390X:
		return "s390x"
	case ARCH_X86_64:
		return "x86_64"
	default:
		panic("invalid architecture")
	}
}

func FromString(a string) Arch {
	switch a {
	case "amd64":
		fallthrough
	case "x86_64":
		return ARCH_X86_64
	case "arm64":
		fallthrough
	case "aarch64":
		return ARCH_AARCH64
	case "s390x":
		return ARCH_S390X
	case "ppc64le":
		return ARCH_PPC64LE
	default:
		panic("unsupported architecture")
	}
}

var runtimeGOARCH = runtime.GOARCH

func Current() Arch {
	switch runtimeGOARCH {
	case "amd64":
		return ARCH_X86_64
	case "arm64":
		return ARCH_AARCH64
	case "ppc64le":
		return ARCH_PPC64LE
	case "s390x":
		return ARCH_S390X
	default:
		panic("unsupported architecture")
	}
}

func IsX86_64() bool {
	return Current() == ARCH_X86_64
}

func IsAarch64() bool {
	return Current() == ARCH_AARCH64
}

func IsPPC() bool {
	return Current() == ARCH_PPC64LE
}

func IsS390x() bool {
	return Current() == ARCH_S390X
}
