// Wire tags from TrueBrain/blue-marshal-rs (fa997642); see LICENSE.
package bluemarshal

const (
	TY_INVALID    byte = 0
	TY_SIGNATURE  byte = 126
	TY_SIGNATURE2 byte = 125
	TY_NONE       byte = 1
	TY_GLOBAL     byte = 2
	TY_INT64      byte = 3
	TY_INT32      byte = 4
	TY_INT16      byte = 5
	TY_INT8       byte = 6
	TY_INT_N1     byte = 7
	TY_INT_0      byte = 8
	TY_INT_1      byte = 9
	TY_FLOAT      byte = 10
	TY_FLOAT_0    byte = 11
	TY_COMPLEX    byte = 12
	TY_STR        byte = 13
	TY_STR_EMPTY  byte = 14
	TY_STR_CHAR   byte = 15
	TY_STR_SHORT  byte = 16
	TY_STR_TABLE  byte = 17
	TY_UNICODE    byte = 18
	TY_BUFFER     byte = 19
	TY_TUPLE      byte = 20
	TY_LIST       byte = 21
	TY_DICT       byte = 22
	TY_INSTANCE   byte = 23
	TY_CALLBACK   byte = 25
	TY_PICKLE     byte = 26
	TY_REFERENCE  byte = 27
	TY_CRC_CHECK  byte = 28
	TY_TRUE       byte = 31
	TY_FALSE      byte = 32
	TY_PICKLER    byte = 33
	TY_REDUCE     byte = 34
	TY_NEWOBJ     byte = 35
	TY_TUPLE0     byte = 36
	TY_TUPLE1     byte = 37
	TY_LIST0      byte = 38
	TY_LIST1      byte = 39
	TY_UNICODE_0  byte = 40
	TY_UNICODE_1  byte = 41
	TY_DBROW      byte = 42
	TY_WSTREAM    byte = 43
	TY_TUPLE2     byte = 44
	TY_MARK       byte = 45
	TY_UTF8       byte = 46
	TY_LONG       byte = 47
	TY_SHAREDFLAG byte = 0x40
	TY_TYPEMASK   byte = 0x3F
)
