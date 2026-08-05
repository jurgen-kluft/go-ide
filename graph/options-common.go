package graph

type CppStdType uint8

const (
	CppStdDefault CppStdType = iota
	CppStd11
	CppStd14
	CppStd17
	CppStd20
	CppStdLatest
)

type CStdType uint8

const (
	CStdDefault CStdType = iota
	CStd11
	CStd17
	CStdLatest
)

func EnsureCppStd(requested CppStdType, defaultValue CppStdType) CppStdType {
	if requested == CppStdDefault {
		return defaultValue
	}
	return requested
}

func EnsureCStd(requested CStdType, defaultValue CStdType) CStdType {
	if requested == CStdDefault {
		return defaultValue
	}
	return requested
}
