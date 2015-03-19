#include <stdio.h>

#define LINE_COUNTER_BUF_LEN 20
static char line_buf[LINE_COUNTER_BUF_LEN] =
{
	' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ',
	' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', '0',
	'\t', '\0'
};

/* Position in 'line_buf' where printing starts.  This will not change
	 unless the number of lines is larger than 999999.  */
static char *line_num_print = line_buf + LINE_COUNTER_BUF_LEN - 8;

/* Position of the first digit in 'line_buf'.  */
static char *line_num_start = line_buf + LINE_COUNTER_BUF_LEN - 3;

/* Position of the last digit in 'line_buf'.  */
static char *line_num_end = line_buf + LINE_COUNTER_BUF_LEN - 3;

static void
compute_line_num()
{
	char *endp = line_num_end;
	do
		{
			if ((*endp)++ < '9')
				return;
			printf("-%d\n", *endp);
			*endp-- = '0';
			printf(" %s\n", endp);
		}
	while (endp >= line_num_start);
	if (line_num_start > line_buf) {
		*--line_num_start = '1';
	} else {
		*line_buf = '>';
	}

	if (line_num_start < line_num_print)
		line_num_print--;
}

int
main()
{
	int i;
	int count = 100;
	for (i = 0; i < count; ++i)
	{
		compute_line_num();
		//printf("%s\n", line_num_print);
	}

	return 0;
}