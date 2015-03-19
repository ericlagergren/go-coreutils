#include <stdio.h>
#include <stdlib.h>

static void
close_stdout(void)
{
	if (fclose (stdout) != 0)
	{
 		perror ("hello: write error");
 		exit (EXIT_FAILURE);
 	}
}

int
main()
{
	atexit (close_stdout);
	printf ("Hello, world!\n");
	return EXIT_SUCCESS;
}