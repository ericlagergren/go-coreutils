/* kwsearch.c - searching subroutines using kwset for grep.
   Copyright 1992, 1998, 2000, 2007, 2009-2015 Free Software Foundation, Inc.

   This program is free software; you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation; either version 3, or (at your option)
   any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program; if not, write to the Free Software
   Foundation, Inc., 51 Franklin Street - Fifth Floor, Boston, MA
   02110-1301, USA.  */

/* Written August 1992 by Mike Haertel. */

#include <config.h>
#include "search.h"

/* Whether -w considers WC to be a word constituent.  */
static bool
wordchar (wint_t wc)
{
  return wc == L'_' || iswalnum (wc);
}

/* KWset compiled pattern.  For Ecompile and Gcompile, we compile
   a list of strings, at least one of which is known to occur in
   any string matching the regexp. */
static kwset_t kwset;

void
Fcompile (char const *pattern, size_t size)
{
  size_t total = size;
  mb_len_map_t *map = NULL;
  char const *pat = (match_icase && MB_CUR_MAX > 1
                     ? mbtoupper (pattern, &total, &map)
                     : pattern);

  kwsinit (&kwset);

  char const *p = pat;
  do
    {
      size_t len;
      char const *sep = memchr (p, '\n', total);
      if (sep)
        {
          len = sep - p;
          sep++;
          total -= (len + 1);
        }
      else
        {
          len = total;
          total = 0;
        }

      char *buf = NULL;
      if (match_lines)
        {
          buf = xmalloc (len + 2);
          buf[0] = eolbyte;
          memcpy (buf + 1, p, len);
          buf[len + 1] = eolbyte;
          p = buf;
          len += 2;
        }
      kwsincr (kwset, p, len);
      free (buf);

      p = sep;
    }
  while (p);

  kwsprep (kwset);
}

/* Apply the MAP (created by mbtoupper) to the uppercase-buffer-relative
   *OFF and *LEN, converting them to be relative to the original buffer.  */

static void
mb_case_map_apply (mb_len_map_t const *map, size_t *off, size_t *len)
{
  if (map)
    {
      size_t off_incr = 0;
      size_t len_incr = 0;
      size_t k;
      for (k = 0; k < *off; k++)
        off_incr += map[k];
      for (; k < *off + *len; k++)
        len_incr += map[k];
      *off += off_incr;
      *len += len_incr;
    }
}

size_t
Fexecute (char const *buf, size_t size, size_t *match_size,
          char const *start_ptr)
{
  char const *beg, *try, *end, *mb_start;
  size_t len;
  char eol = eolbyte;
  struct kwsmatch kwsmatch;
  size_t ret_val;
  mb_len_map_t *map = NULL;

  if (MB_CUR_MAX > 1)
    {
      if (match_icase)
        {
          char *case_buf = mbtoupper (buf, &size, &map);
          if (start_ptr)
            start_ptr = case_buf + (start_ptr - buf);
          buf = case_buf;
        }
    }

  for (mb_start = beg = start_ptr ? start_ptr : buf; beg <= buf + size; beg++)
    {
      size_t offset = kwsexec (kwset, beg - match_lines,
                               buf + size - beg + match_lines, &kwsmatch);
      if (offset == (size_t) -1)
        goto failure;
      len = kwsmatch.size[0] - 2 * match_lines;
      if (!match_lines && MB_CUR_MAX > 1 && !using_utf8 ()
          && mb_goback (&mb_start, beg + offset, buf + size) != 0)
        {
          /* We have matched a single byte that is not at the beginning of a
             multibyte character.  mb_goback has advanced MB_START past that
             multibyte character.  Now, we want to position BEG so that the
             next kwsexec search starts there.  Thus, to compensate for the
             for-loop's BEG++, above, subtract one here.  This code is
             unusually hard to reach, and exceptionally, let's show how to
             trigger it here:

               printf '\203AA\n'|LC_ALL=ja_JP.SHIFT_JIS src/grep -F A

             That assumes the named locale is installed.
             Note that your system's shift-JIS locale may have a different
             name, possibly including "sjis".  */
          beg = mb_start - 1;
          continue;
        }
      beg += offset;
      if (start_ptr && !match_words)
        goto success_in_beg_and_len;
      if (match_lines)
        {
          len += start_ptr == NULL;
          goto success_in_beg_and_len;
        }
      if (match_words)
        for (try = beg; ; )
          {
            if (wordchar (mb_prev_wc (buf, try, buf + size)))
              break;
            if (wordchar (mb_next_wc (try + len, buf + size)))
              {
                if (!len)
                  break;
                offset = kwsexec (kwset, beg, --len, &kwsmatch);
                if (offset == (size_t) -1)
                  break;
                try = beg + offset;
                len = kwsmatch.size[0];
              }
            else if (!start_ptr)
              goto success;
            else
              goto success_in_beg_and_len;
          } /* for (try) */
      else
        goto success;
    } /* for (beg in buf) */

 failure:
  return -1;

 success:
  if ((end = memchr (beg + len, eol, (buf + size) - (beg + len))) != NULL)
    end++;
  else
    end = buf + size;
  while (buf < beg && beg[-1] != eol)
    --beg;
  len = end - beg;
 success_in_beg_and_len:;
  size_t off = beg - buf;
  mb_case_map_apply (map, &off, &len);

  *match_size = len;
  ret_val = off;
  return ret_val;
}