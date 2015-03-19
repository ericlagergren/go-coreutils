#include <stdio.h>
#include <sys/types.h>

#define CHAR_BIT 8
#define TYPE_SIGNED(type) (((type) -1) < 0)
#define INT_STRLEN_BOUND(t) ((sizeof (t) * CHAR_BIT - TYPE_SIGNED (t)) * 302 / 1000 + 1 + TYPE_SIGNED (t))

int
main()
{
	char a[INT_STRLEN_BOUND (pid_t)];

	printf("%zu\n", a);
	return(0);
}