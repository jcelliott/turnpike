package turnpike

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestToLength(t *testing.T) {
	// exhaustive list of valid lengths
	exp := map[byte]int{
		0:  2 << 8,
		1:  2 << 9,
		2:  2 << 10,
		3:  2 << 11,
		4:  2 << 12,
		5:  2 << 13,
		6:  2 << 14,
		7:  2 << 15,
		8:  2 << 16,
		9:  2 << 17,
		10: 2 << 18,
		11: 2 << 19,
		12: 2 << 20,
		13: 2 << 21,
		14: 2 << 22,
		15: 2 << 23,
	}

	Convey("For every valid length value", t, func() {
		for b, v := range exp {
			So(toLength(b), ShouldEqual, v)
		}
	})
}

func TestIntToBytes(t *testing.T) {
	Convey("When setting a number that fits in a byte", t, func() {
		val := 56
		arr := intToBytes(val)
		So(len(arr), ShouldEqual, 3)
		So(arr[0], ShouldEqual, 0)
		So(arr[1], ShouldEqual, 0)
		So(arr[2], ShouldEqual, val)
	})

	Convey("When setting a number that fits in a 24-bit number", t, func() {
		val := 2 << 20
		arr := intToBytes(val)
		So(len(arr), ShouldEqual, 3)
		So(arr[0], ShouldEqual, val>>16)
		So(arr[1], ShouldEqual, 0)
		So(arr[2], ShouldEqual, 0)
	})
}

func TestBytesToInt(t *testing.T) {
	Convey("When setting an array with only the low byte set", t, func() {
		arr := []byte{0, 0, 56}
		val := bytesToInt(arr)
		So(val, ShouldEqual, arr[2])
	})
	Convey("When setting an array with only the high byte set", t, func() {
		arr := []byte{56, 0, 0}
		val := bytesToInt(arr)
		So(val, ShouldEqual, uint(arr[0])<<16)
	})
}
