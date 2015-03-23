package main

import (
	"os"
	//"syscall"
	"fmt"
	"unsafe"
)

var (
	IOCPARM_MAX = os.Getpagesize() /* max size of ioctl, mult. of NBPG */
)

const (
	_n          = int(0)
	IOC_VOID    = 0x20000000 /* no parameters */
	IOC_OUT     = 0x40000000 /* copy out parameters */
	IOC_IN      = 0x80000000 /* copy in parameters */
	IOC_INOUT   = (IOC_IN | IOC_OUT)
	IOC_DIRMASK = 0xe0000000 /* mask for IN/OUT/VOID */
)

/*
#define _IOC(inout,group,num,len) \
	(inout | ((len & IOCPARM_MASK) << 16) | ((group) << 8) | (num))
#define	_IO(g,n)	_IOC(IOC_VOID,	(g), (n), 0)
#define	_IOR(g,n,t)	_IOC(IOC_OUT,	(g), (n), sizeof(t))
#define	_IOW(g,n,t)	_IOC(IOC_IN,	(g), (n), sizeof(t))
// this should be _IORW, but stdio got there first
#define	_IOWR(g,n,t)	_IOC(IOC_INOUT,	(g), (n), sizeof(t)) */

//#define	FIONREAD	_IOR('f', 127, int)	/* get # bytes to read */

var FIONREAD = (IOC_OUT | ((unsafe.Sizeof(_n) & uintptr(IOCPARM_MAX)) << 16) | (('f') << 8) | (127))

func ioctl(fd uintptr, request uintptr, arg *uintptr) {
	fmt.Println(FIONREAD)
}

func main() {
	file, err := os.Open("test.c")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	n := uintptr(0)
	ioctl(file.Fd(), FIONREAD, &n)
}
