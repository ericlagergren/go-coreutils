#include <stdio.h>
#include <sys/ioctl.h>

#define _ioc(inout,group,num,len) \
	(inout | ((len & IOCPARM_MASK) << 16) | ((group) << 8) | (num))
#define	_ior(g,n,t)	_IOC(IOC_OUT,	(g), (n), sizeof(t))
#define fionread _ior('f', 127, int)


int
main()
{
	#ifdef FIONREAD
		printf(">> %d\n", FIONREAD);
	#endif

	printf("%lu\n", fionread);

	return 0;
}