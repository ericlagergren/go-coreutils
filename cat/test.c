#include <stdio.h>
#include <stdlib.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <unistd.h>

int
main(int argc, char const *argv[])
{

	(void) argc;

	const char *file = argv[1];
	
	struct stat fs;
	stat(file, &fs);
	dev_t file_dev = fs.st_dev;
	ino_t file_ino = fs.st_ino;

	struct stat os;
	fstat(STDOUT_FILENO, &os);
	dev_t out_dev = os.st_dev;
	ino_t out_ino = os.st_ino;

	if (file_dev == out_dev && file_ino == out_ino) {
		printf("%s\n", "same file!");
	}

	return 0;
}