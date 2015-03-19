/* dfa.c - deterministic extended regexp routines for GNU
   Copyright (C) 1988, 1998, 2000, 2002, 2004-2005, 2007-2015 Free Software
   Foundation, Inc.

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
   Foundation, Inc.,
   51 Franklin Street - Fifth Floor, Boston, MA  02110-1301, USA */

/* Written June, 1988 by Mike Haertel
   Modified July, 1988 by Arthur David Olson to assist BMG speedups  */

#include <config.h>

#include "dfa.h"

#include <assert.h>
#include <ctype.h>
#include <stdio.h>
#include <stdlib.h>
#include <limits.h>
#include <string.h>
#include <locale.h>

#define STREQ(a, b) (strcmp (a, b) == 0)

/* ISASCIIDIGIT differs from isdigit, as follows:
   - Its arg may be any int or unsigned int; it need not be an unsigned char.
   - It's guaranteed to evaluate its argument exactly once.
   - It's typically faster.
   Posix 1003.2-1992 section 2.5.2.1 page 50 lines 1556-1558 says that
   only '0' through '9' are digits.  Prefer ISASCIIDIGIT to isdigit unless
   it's important to use the locale's definition of "digit" even when the
   host does not conform to Posix.  */
#define ISASCIIDIGIT(c) ((unsigned) (c) - '0' <= 9)

#include "gettext.h"
#define _(str) gettext (str)

#include <wchar.h>
#include <wctype.h>

#include "xalloc.h"

/* HPUX defines these as macros in sys/param.h.  */
#ifdef setbit
# undef setbit
#endif
#ifdef clrbit
# undef clrbit
#endif

/* First integer value that is greater than any character code.  */
enum { NOTCHAR = 1 << CHAR_BIT };

/* This represents part of a character class.  It must be unsigned and
   at least CHARCLASS_WORD_BITS wide.  Any excess bits are zero.  */
typedef unsigned int charclass_word;

/* The number of bits used in a charclass word.  utf8_classes assumes
   this is exactly 32.  */
enum { CHARCLASS_WORD_BITS = 32 };

/* The maximum useful value of a charclass_word; all used bits are 1.  */
#define CHARCLASS_WORD_MASK \
  (((charclass_word) 1 << (CHARCLASS_WORD_BITS - 1) << 1) - 1)

/* Number of words required to hold a bit for every character.  */
enum
{
  CHARCLASS_WORDS = (NOTCHAR + CHARCLASS_WORD_BITS - 1) / CHARCLASS_WORD_BITS
};

/* Sets of unsigned characters are stored as bit vectors in arrays of ints.  */
typedef charclass_word charclass[CHARCLASS_WORDS];

/* Convert a possibly-signed character to an unsigned character.  This is
   a bit safer than casting to unsigned char, since it catches some type
   errors that the cast doesn't.  */
static unsigned char
to_uchar (char ch)
{
  return ch;
}

/* Contexts tell us whether a character is a newline or a word constituent.
   Word-constituent characters are those that satisfy iswalnum, plus '_'.
   Each character has a single CTX_* value; bitmasks of CTX_* values denote
   a particular character class.

   A state also stores a context value, which is a bitmask of CTX_* values.
   A state's context represents a set of characters that the state's
   predecessors must match.  For example, a state whose context does not
   include CTX_LETTER will never have transitions where the previous
   character is a word constituent.  A state whose context is CTX_ANY
   might have transitions from any character.  */

#define CTX_NONE	1
#define CTX_LETTER	2
#define CTX_NEWLINE	4
#define CTX_ANY		7

/* Sometimes characters can only be matched depending on the surrounding
   context.  Such context decisions depend on what the previous character
   was, and the value of the current (lookahead) character.  Context
   dependent constraints are encoded as 8 bit integers.  Each bit that
   is set indicates that the constraint succeeds in the corresponding
   context.

   bit 8-11 - valid contexts when next character is CTX_NEWLINE
   bit 4-7  - valid contexts when next character is CTX_LETTER
   bit 0-3  - valid contexts when next character is CTX_NONE

   The macro SUCCEEDS_IN_CONTEXT determines whether a given constraint
   succeeds in a particular context.  Prev is a bitmask of possible
   context values for the previous character, curr is the (single-bit)
   context value for the lookahead character.  */
#define NEWLINE_CONSTRAINT(constraint) (((constraint) >> 8) & 0xf)
#define LETTER_CONSTRAINT(constraint)  (((constraint) >> 4) & 0xf)
#define OTHER_CONSTRAINT(constraint)    ((constraint)       & 0xf)

#define SUCCEEDS_IN_CONTEXT(constraint, prev, curr) \
  ((((curr) & CTX_NONE      ? OTHER_CONSTRAINT (constraint) : 0) \
    | ((curr) & CTX_LETTER  ? LETTER_CONSTRAINT (constraint) : 0) \
    | ((curr) & CTX_NEWLINE ? NEWLINE_CONSTRAINT (constraint) : 0)) & (prev))

/* The following macros describe what a constraint depends on.  */
#define PREV_NEWLINE_CONSTRAINT(constraint) (((constraint) >> 2) & 0x111)
#define PREV_LETTER_CONSTRAINT(constraint)  (((constraint) >> 1) & 0x111)
#define PREV_OTHER_CONSTRAINT(constraint)    ((constraint)       & 0x111)

#define PREV_NEWLINE_DEPENDENT(constraint) \
  (PREV_NEWLINE_CONSTRAINT (constraint) != PREV_OTHER_CONSTRAINT (constraint))
#define PREV_LETTER_DEPENDENT(constraint) \
  (PREV_LETTER_CONSTRAINT (constraint) != PREV_OTHER_CONSTRAINT (constraint))

/* Tokens that match the empty string subject to some constraint actually
   work by applying that constraint to determine what may follow them,
   taking into account what has gone before.  The following values are
   the constraints corresponding to the special tokens previously defined.  */
#define NO_CONSTRAINT         0x777
#define BEGLINE_CONSTRAINT    0x444
#define ENDLINE_CONSTRAINT    0x700
#define BEGWORD_CONSTRAINT    0x050
#define ENDWORD_CONSTRAINT    0x202
#define LIMWORD_CONSTRAINT    0x252
#define NOTLIMWORD_CONSTRAINT 0x525

/* The regexp is parsed into an array of tokens in postfix form.  Some tokens
   are operators and others are terminal symbols.  Most (but not all) of these
   codes are returned by the lexical analyzer.  */

typedef ptrdiff_t token;

/* Predefined token values.  */
enum
{
  END = -1,                     /* END is a terminal symbol that matches the
                                   end of input; any value of END or less in
                                   the parse tree is such a symbol.  Accepting
                                   states of the DFA are those that would have
                                   a transition on END.  */

  /* Ordinary character values are terminal symbols that match themselves.  */

  EMPTY = NOTCHAR,              /* EMPTY is a terminal symbol that matches
                                   the empty string.  */

  BACKREF,                      /* BACKREF is generated by \<digit>
                                   or by any other construct that
                                   is not completely handled.  If the scanner
                                   detects a transition on backref, it returns
                                   a kind of "semi-success" indicating that
                                   the match will have to be verified with
                                   a backtracking matcher.  */

  BEGLINE,                      /* BEGLINE is a terminal symbol that matches
                                   the empty string at the beginning of a
                                   line.  */

  ENDLINE,                      /* ENDLINE is a terminal symbol that matches
                                   the empty string at the end of a line.  */

  BEGWORD,                      /* BEGWORD is a terminal symbol that matches
                                   the empty string at the beginning of a
                                   word.  */

  ENDWORD,                      /* ENDWORD is a terminal symbol that matches
                                   the empty string at the end of a word.  */

  LIMWORD,                      /* LIMWORD is a terminal symbol that matches
                                   the empty string at the beginning or the
                                   end of a word.  */

  NOTLIMWORD,                   /* NOTLIMWORD is a terminal symbol that
                                   matches the empty string not at
                                   the beginning or end of a word.  */

  QMARK,                        /* QMARK is an operator of one argument that
                                   matches zero or one occurrences of its
                                   argument.  */

  STAR,                         /* STAR is an operator of one argument that
                                   matches the Kleene closure (zero or more
                                   occurrences) of its argument.  */

  PLUS,                         /* PLUS is an operator of one argument that
                                   matches the positive closure (one or more
                                   occurrences) of its argument.  */

  REPMN,                        /* REPMN is a lexical token corresponding
                                   to the {m,n} construct.  REPMN never
                                   appears in the compiled token vector.  */

  CAT,                          /* CAT is an operator of two arguments that
                                   matches the concatenation of its
                                   arguments.  CAT is never returned by the
                                   lexical analyzer.  */

  OR,                           /* OR is an operator of two arguments that
                                   matches either of its arguments.  */

  LPAREN,                       /* LPAREN never appears in the parse tree,
                                   it is only a lexeme.  */

  RPAREN,                       /* RPAREN never appears in the parse tree.  */

  ANYCHAR,                      /* ANYCHAR is a terminal symbol that matches
                                   a valid multibyte (or single byte) character.
                                   It is used only if MB_CUR_MAX > 1.  */

  MBCSET,                       /* MBCSET is similar to CSET, but for
                                   multibyte characters.  */

  WCHAR,                        /* Only returned by lex.  wctok contains
                                   the wide character representation.  */

  CSET                          /* CSET and (and any value greater) is a
                                   terminal symbol that matches any of a
                                   class of characters.  */
};


/* States of the recognizer correspond to sets of positions in the parse
   tree, together with the constraints under which they may be matched.
   So a position is encoded as an index into the parse tree together with
   a constraint.  */
typedef struct
{
  size_t index;                 /* Index into the parse array.  */
  unsigned int constraint;      /* Constraint for matching this position.  */
} position;

/* Sets of positions are stored as arrays.  */
typedef struct
{
  position *elems;              /* Elements of this position set.  */
  size_t nelem;                 /* Number of elements in this set.  */
  size_t alloc;                 /* Number of elements allocated in ELEMS.  */
} position_set;

/* Sets of leaves are also stored as arrays.  */
typedef struct
{
  size_t *elems;                /* Elements of this position set.  */
  size_t nelem;                 /* Number of elements in this set.  */
} leaf_set;

/* A state of the dfa consists of a set of positions, some flags,
   and the token value of the lowest-numbered position of the state that
   contains an END token.  */
typedef struct
{
  size_t hash;                  /* Hash of the positions of this state.  */
  position_set elems;           /* Positions this state could match.  */
  unsigned char context;        /* Context from previous state.  */
  bool has_backref;		/* This state matches a \<digit>.  */
  bool has_mbcset;		/* This state matches a MBCSET.  */
  unsigned short constraint;    /* Constraint for this state to accept.  */
  token first_end;              /* Token value of the first END in elems.  */
  position_set mbps;            /* Positions which can match multibyte
                                   characters, e.g., period.
                                   Used only if MB_CUR_MAX > 1.  */
} dfa_state;

/* States are indexed by state_num values.  These are normally
   nonnegative but -1 is used as a special value.  */
typedef ptrdiff_t state_num;

/* A bracket operator.
   e.g., [a-c], [[:alpha:]], etc.  */
struct mb_char_classes
{
  ptrdiff_t cset;
  bool invert;
  wchar_t *chars;               /* Normal characters.  */
  size_t nchars;
  wctype_t *ch_classes;         /* Character classes.  */
  size_t nch_classes;
  struct			/* Range characters.  */
  {
    wchar_t beg;		/* Range start.  */
    wchar_t end;		/* Range end.  */
  } *ranges;
  size_t nranges;
  char **equivs;                /* Equivalence classes.  */
  size_t nequivs;
  char **coll_elems;
  size_t ncoll_elems;           /* Collating elements.  */
};

/* A compiled regular expression.  */
struct dfa
{
  /* Fields filled by the scanner.  */
  charclass *charclasses;       /* Array of character sets for CSET tokens.  */
  size_t cindex;                /* Index for adding new charclasses.  */
  size_t calloc;                /* Number of charclasses allocated.  */

  /* Fields filled by the parser.  */
  token *tokens;                /* Postfix parse array.  */
  size_t tindex;                /* Index for adding new tokens.  */
  size_t talloc;                /* Number of tokens currently allocated.  */
  size_t depth;                 /* Depth required of an evaluation stack
                                   used for depth-first traversal of the
                                   parse tree.  */
  size_t nleaves;               /* Number of leaves on the parse tree.  */
  size_t nregexps;              /* Count of parallel regexps being built
                                   with dfaparse.  */
  bool fast;			/* The DFA is fast.  */
  bool multibyte;		/* MB_CUR_MAX > 1.  */
  token utf8_anychar_classes[5]; /* To lower ANYCHAR in UTF-8 locales.  */
  mbstate_t mbs;		/* Multibyte conversion state.  */

  /* dfaexec implementation.  */
  char *(*dfaexec) (struct dfa *, char const *, char *, int, size_t *, int *);

  /* The following are valid only if MB_CUR_MAX > 1.  */

  /* The value of multibyte_prop[i] is defined by following rule.
     if tokens[i] < NOTCHAR
     bit 0 : tokens[i] is the first byte of a character, including
     single-byte characters.
     bit 1 : tokens[i] is the last byte of a character, including
     single-byte characters.

     if tokens[i] = MBCSET
     ("the index of mbcsets corresponding to this operator" << 2) + 3

     e.g.
     tokens
     = 'single_byte_a', 'multi_byte_A', single_byte_b'
     = 'sb_a', 'mb_A(1st byte)', 'mb_A(2nd byte)', 'mb_A(3rd byte)', 'sb_b'
     multibyte_prop
     = 3     , 1               ,  0              ,  2              , 3
   */
  int *multibyte_prop;

  /* A table indexed by byte values that contains the corresponding wide
     character (if any) for that byte.  WEOF means the byte is not a
     valid single-byte character.  */
  wint_t mbrtowc_cache[NOTCHAR];

  /* Array of the bracket expression in the DFA.  */
  struct mb_char_classes *mbcsets;
  size_t nmbcsets;
  size_t mbcsets_alloc;

  /* Fields filled by the superset.  */
  struct dfa *superset;             /* Hint of the dfa.  */

  /* Fields filled by the state builder.  */
  dfa_state *states;            /* States of the dfa.  */
  state_num sindex;             /* Index for adding new states.  */
  size_t salloc;		/* Number of states currently allocated.  */

  /* Fields filled by the parse tree->NFA conversion.  */
  position_set *follows;        /* Array of follow sets, indexed by position
                                   index.  The follow of a position is the set
                                   of positions containing characters that
                                   could conceivably follow a character
                                   matching the given position in a string
                                   matching the regexp.  Allocated to the
                                   maximum possible position index.  */
  bool searchflag;		/* We are supposed to build a searching
                                   as opposed to an exact matcher.  A searching
                                   matcher finds the first and shortest string
                                   matching a regexp anywhere in the buffer,
                                   whereas an exact matcher finds the longest
                                   string matching, but anchored to the
                                   beginning of the buffer.  */

  /* Fields filled by dfaexec.  */
  state_num tralloc;            /* Number of transition tables that have
                                   slots so far, not counting trans[-1].  */
  int trcount;                  /* Number of transition tables that have
                                   actually been built.  */
  int min_trcount;              /* Minimum of number of transition tables.
                                   Always keep the number, even after freeing
                                   the transition tables.  It is also the
                                   number of initial states.  */
  state_num **trans;            /* Transition tables for states that can
                                   never accept.  If the transitions for a
                                   state have not yet been computed, or the
                                   state could possibly accept, its entry in
                                   this table is NULL.  This points to one
                                   past the start of the allocated array,
                                   and trans[-1] is always NULL.  */
  state_num **fails;            /* Transition tables after failing to accept
                                   on a state that potentially could do so.  */
  int *success;                 /* Table of acceptance conditions used in
                                   dfaexec and computed in build_state.  */
  state_num *newlines;          /* Transitions on newlines.  The entry for a
                                   newline in any transition table is always
                                   -1 so we can count lines without wasting
                                   too many cycles.  The transition for a
                                   newline is stored separately and handled
                                   as a special case.  Newline is also used
                                   as a sentinel at the end of the buffer.  */
  state_num initstate_letter;   /* Initial state for letter context.  */
  state_num initstate_others;   /* Initial state for other contexts.  */
  struct dfamust *musts;        /* List of strings, at least one of which
                                   is known to appear in any r.e. matching
                                   the dfa.  */
  position_set mb_follows;	/* Follow set added by ANYCHAR and/or MBCSET
                                   on demand.  */
  int *mb_match_lens;           /* Array of length reduced by ANYCHAR and/or
                                   MBCSET.  Null if mb_follows.elems has not
                                   been allocated.  */
};

/* Some macros for user access to dfa internals.  */

/* S could possibly be an accepting state of R.  */
#define ACCEPTING(s, r) ((r).states[s].constraint)

/* STATE accepts in the specified context.  */
#define ACCEPTS_IN_CONTEXT(prev, curr, state, dfa) \
  SUCCEEDS_IN_CONTEXT ((dfa).states[state].constraint, prev, curr)

static void dfamust (struct dfa *dfa);
static void regexp (void);

static void
dfambcache (struct dfa *d)
{
  int i;
  for (i = CHAR_MIN; i <= CHAR_MAX; ++i)
    {
      char c = i;
      unsigned char uc = i;
      mbstate_t s = { 0 };
      wchar_t wc;
      d->mbrtowc_cache[uc] = mbrtowc (&wc, &c, 1, &s) <= 1 ? wc : WEOF;
    }
}

/* Store into *PWC the result of converting the leading bytes of the
   multibyte buffer S of length N bytes, using the mbrtowc_cache in *D
   and updating the conversion state in *D.  On conversion error,
   convert just a single byte, to WEOF.  Return the number of bytes
   converted.

   This differs from mbrtowc (PWC, S, N, &D->mbs) as follows:

   * PWC points to wint_t, not to wchar_t.
   * The last arg is a dfa *D instead of merely a multibyte conversion
     state D->mbs.  D also contains an mbrtowc_cache for speed.
   * N must be at least 1.
   * S[N - 1] must be a sentinel byte.
   * Shift encodings are not supported.
   * The return value is always in the range 1..N.
   * D->mbs is always valid afterwards.
   * *PWC is always set to something.  */
static size_t
mbs_to_wchar (wint_t *pwc, char const *s, size_t n, struct dfa *d)
{
  unsigned char uc = s[0];
  wint_t wc = d->mbrtowc_cache[uc];

  if (wc == WEOF)
    {
      wchar_t wch;
      size_t nbytes = mbrtowc (&wch, s, n, &d->mbs);
      if (0 < nbytes && nbytes < (size_t) -2)
        {
          *pwc = wch;
          return nbytes;
        }
      memset (&d->mbs, 0, sizeof d->mbs);
    }

  *pwc = wc;
  return 1;
}

#ifdef DEBUG

static void
prtok (token t)
{
  char const *s;

  if (t < 0)
    fprintf (stderr, "END");
  else if (t < NOTCHAR)
    {
      int ch = t;
      fprintf (stderr, "%c", ch);
    }
  else
    {
      switch (t)
        {
        case EMPTY:
          s = "EMPTY";
          break;
        case BACKREF:
          s = "BACKREF";
          break;
        case BEGLINE:
          s = "BEGLINE";
          break;
        case ENDLINE:
          s = "ENDLINE";
          break;
        case BEGWORD:
          s = "BEGWORD";
          break;
        case ENDWORD:
          s = "ENDWORD";
          break;
        case LIMWORD:
          s = "LIMWORD";
          break;
        case NOTLIMWORD:
          s = "NOTLIMWORD";
          break;
        case QMARK:
          s = "QMARK";
          break;
        case STAR:
          s = "STAR";
          break;
        case PLUS:
          s = "PLUS";
          break;
        case CAT:
          s = "CAT";
          break;
        case OR:
          s = "OR";
          break;
        case LPAREN:
          s = "LPAREN";
          break;
        case RPAREN:
          s = "RPAREN";
          break;
        case ANYCHAR:
          s = "ANYCHAR";
          break;
        case MBCSET:
          s = "MBCSET";
          break;
        default:
          s = "CSET";
          break;
        }
      fprintf (stderr, "%s", s);
    }
}
#endif /* DEBUG */

/* Stuff pertaining to charclasses.  */

static bool
tstbit (unsigned int b, charclass const c)
{
  return c[b / CHARCLASS_WORD_BITS] >> b % CHARCLASS_WORD_BITS & 1;
}

static void
setbit (unsigned int b, charclass c)
{
  c[b / CHARCLASS_WORD_BITS] |= (charclass_word) 1 << b % CHARCLASS_WORD_BITS;
}

static void
clrbit (unsigned int b, charclass c)
{
  c[b / CHARCLASS_WORD_BITS] &= ~((charclass_word) 1
                                  << b % CHARCLASS_WORD_BITS);
}

static void
copyset (charclass const src, charclass dst)
{
  memcpy (dst, src, sizeof (charclass));
}

static void
zeroset (charclass s)
{
  memset (s, 0, sizeof (charclass));
}

static void
notset (charclass s)
{
  int i;

  for (i = 0; i < CHARCLASS_WORDS; ++i)
    s[i] = CHARCLASS_WORD_MASK & ~s[i];
}

static bool
equal (charclass const s1, charclass const s2)
{
  return memcmp (s1, s2, sizeof (charclass)) == 0;
}

/* Ensure that the array addressed by PTR holds at least NITEMS +
   (PTR || !NITEMS) items.  Either return PTR, or reallocate the array
   and return its new address.  Although PTR may be null, the returned
   value is never null.

   The array holds *NALLOC items; *NALLOC is updated on reallocation.
   ITEMSIZE is the size of one item.  Avoid O(N**2) behavior on arrays
   growing linearly.  */
static void *
maybe_realloc (void *ptr, size_t nitems, size_t *nalloc, size_t itemsize)
{
  if (nitems < *nalloc)
    return ptr;
  *nalloc = nitems;
  return x2nrealloc (ptr, nalloc, itemsize);
}

/* In DFA D, find the index of charclass S, or allocate a new one.  */
static size_t
dfa_charclass_index (struct dfa *d, charclass const s)
{
  size_t i;

  for (i = 0; i < d->cindex; ++i)
    if (equal (s, d->charclasses[i]))
      return i;
  d->charclasses = maybe_realloc (d->charclasses, d->cindex, &d->calloc,
                                  sizeof *d->charclasses);
  ++d->cindex;
  copyset (s, d->charclasses[i]);
  return i;
}

/* A pointer to the current dfa is kept here during parsing.  */
static struct dfa *dfa;

/* Find the index of charclass S in the current DFA, or allocate a new one.  */
static size_t
charclass_index (charclass const s)
{
  return dfa_charclass_index (dfa, s);
}

/* Syntax bits controlling the behavior of the lexical analyzer.  */
static reg_syntax_t syntax_bits, syntax_bits_set;

/* Flag for case-folding letters into sets.  */
static bool case_fold;

/* End-of-line byte in data.  */
static unsigned char eolbyte;

/* Cache of char-context values.  */
static int sbit[NOTCHAR];

/* Set of characters considered letters.  */
static charclass letters;

/* Set of characters that are newline.  */
static charclass newline;

/* Add this to the test for whether a byte is word-constituent, since on
   BSD-based systems, many values in the 128..255 range are classified as
   alphabetic, while on glibc-based systems, they are not.  */
#ifdef __GLIBC__
# define is_valid_unibyte_character(c) 1
#else
# define is_valid_unibyte_character(c) (btowc (c) != WEOF)
#endif

/* C is a "word-constituent" byte.  */
#define IS_WORD_CONSTITUENT(C) \
  (is_valid_unibyte_character (C) && (isalnum (C) || (C) == '_'))

static int
char_context (unsigned char c)
{
  if (c == eolbyte)
    return CTX_NEWLINE;
  if (IS_WORD_CONSTITUENT (c))
    return CTX_LETTER;
  return CTX_NONE;
}

static int
wchar_context (wint_t wc)
{
  if (wc == (wchar_t) eolbyte || wc == 0)
    return CTX_NEWLINE;
  if (wc == L'_' || iswalnum (wc))
    return CTX_LETTER;
  return CTX_NONE;
}

/* Entry point to set syntax options.  */
void
dfasyntax (reg_syntax_t bits, int fold, unsigned char eol)
{
  unsigned int i;

  syntax_bits_set = 1;
  syntax_bits = bits;
  case_fold = fold != 0;
  eolbyte = eol;

  for (i = 0; i < NOTCHAR; ++i)
    {
      sbit[i] = char_context (i);
      switch (sbit[i])
        {
        case CTX_LETTER:
          setbit (i, letters);
          break;
        case CTX_NEWLINE:
          setbit (i, newline);
          break;
        }
    }
}

/* Set a bit in the charclass for the given wchar_t.  Do nothing if WC
   is represented by a multi-byte sequence.  Even for MB_CUR_MAX == 1,
   this may happen when folding case in weird Turkish locales where
   dotless i/dotted I are not included in the chosen character set.
   Return whether a bit was set in the charclass.  */
static bool
setbit_wc (wint_t wc, charclass c)
{
  int b = wctob (wc);
  if (b == EOF)
    return false;

  setbit (b, c);
  return true;
}

/* Set a bit for B and its case variants in the charclass C.
   MB_CUR_MAX must be 1.  */
static void
setbit_case_fold_c (int b, charclass c)
{
  int ub = toupper (b);
  int i;
  for (i = 0; i < NOTCHAR; i++)
    if (toupper (i) == ub)
      setbit (i, c);
}



/* UTF-8 encoding allows some optimizations that we can't otherwise
   assume in a multibyte encoding.  */
int
using_utf8 (void)
{
  static int utf8 = -1;
  if (utf8 < 0)
    {
      wchar_t wc;
      mbstate_t mbs = { 0 };
      utf8 = mbrtowc (&wc, "\xc4\x80", 2, &mbs) == 2 && wc == 0x100;
    }
  return utf8;
}

/* The current locale is known to be a unibyte locale
   without multicharacter collating sequences and where range
   comparisons simply use the native encoding.  These locales can be
   processed more efficiently.  */

static bool
using_simple_locale (void)
{
  /* The native character set is known to be compatible with
     the C locale.  The following test isn't perfect, but it's good
     enough in practice, as only ASCII and EBCDIC are in common use
     and this test correctly accepts ASCII and rejects EBCDIC.  */
  enum { native_c_charset =
    ('\b' == 8 && '\t' == 9 && '\n' == 10 && '\v' == 11 && '\f' == 12
     && '\r' == 13 && ' ' == 32 && '!' == 33 && '"' == 34 && '#' == 35
     && '%' == 37 && '&' == 38 && '\'' == 39 && '(' == 40 && ')' == 41
     && '*' == 42 && '+' == 43 && ',' == 44 && '-' == 45 && '.' == 46
     && '/' == 47 && '0' == 48 && '9' == 57 && ':' == 58 && ';' == 59
     && '<' == 60 && '=' == 61 && '>' == 62 && '?' == 63 && 'A' == 65
     && 'Z' == 90 && '[' == 91 && '\\' == 92 && ']' == 93 && '^' == 94
     && '_' == 95 && 'a' == 97 && 'z' == 122 && '{' == 123 && '|' == 124
     && '}' == 125 && '~' == 126)
  };

  if (! native_c_charset || dfa->multibyte)
    return false;
  else
    {
      static int unibyte_c = -1;
      if (unibyte_c < 0)
        {
          char const *locale = setlocale (LC_ALL, NULL);
          unibyte_c = (!locale
                       || STREQ (locale, "C")
                       || STREQ (locale, "POSIX"));
        }
      return unibyte_c;
    }
}

/* Lexical analyzer.  All the dross that deals with the obnoxious
   GNU Regex syntax bits is located here.  The poor, suffering
   reader is referred to the GNU Regex documentation for the
   meaning of the @#%!@#%^!@ syntax bits.  */

static char const *lexptr;      /* Pointer to next input character.  */
static size_t lexleft;          /* Number of characters remaining.  */
static token lasttok;           /* Previous token returned; initially END.  */
static bool laststart;		/* We're separated from beginning or (,
                                   | only by zero-width characters.  */
static size_t parens;           /* Count of outstanding left parens.  */
static int minrep, maxrep;      /* Repeat counts for {m,n}.  */

static int cur_mb_len = 1;      /* Length of the multibyte representation of
                                   wctok.  */

static wint_t wctok;		/* Wide character representation of the current
                                   multibyte character, or WEOF if there was
                                   an encoding error.  Used only if
                                   MB_CUR_MAX > 1.  */


/* Fetch the next lexical input character.  Set C (of type int) to the
   next input byte, except set C to EOF if the input is a multibyte
   character of length greater than 1.  Set WC (of type wint_t) to the
   value of the input if it is a valid multibyte character (possibly
   of length 1); otherwise set WC to WEOF.  If there is no more input,
   report EOFERR if EOFERR is not null, and return lasttok = END
   otherwise.  */
# define FETCH_WC(c, wc, eoferr)		\
  do {						\
    if (! lexleft)				\
      {						\
        if ((eoferr) != 0)			\
          dfaerror (eoferr);			\
        else					\
          return lasttok = END;			\
      }						\
    else					\
      {						\
        wint_t _wc;				\
        size_t nbytes = mbs_to_wchar (&_wc, lexptr, lexleft, dfa); \
        cur_mb_len = nbytes;			\
        (wc) = _wc;				\
        (c) = nbytes == 1 ? to_uchar (*lexptr) : EOF;    \
        lexptr += nbytes;			\
        lexleft -= nbytes;			\
      }						\
  } while (0)

#ifndef MIN
# define MIN(a,b) ((a) < (b) ? (a) : (b))
#endif

/* The set of wchar_t values C such that there's a useful locale
   somewhere where C != towupper (C) && C != towlower (towupper (C)).
   For example, 0x00B5 (U+00B5 MICRO SIGN) is in this table, because
   towupper (0x00B5) == 0x039C (U+039C GREEK CAPITAL LETTER MU), and
   towlower (0x039C) == 0x03BC (U+03BC GREEK SMALL LETTER MU).  */
static short const lonesome_lower[] =
  {
    0x00B5, 0x0131, 0x017F, 0x01C5, 0x01C8, 0x01CB, 0x01F2, 0x0345,
    0x03C2, 0x03D0, 0x03D1, 0x03D5, 0x03D6, 0x03F0, 0x03F1,

    /* U+03F2 GREEK LUNATE SIGMA SYMBOL lacks a specific uppercase
       counterpart in locales predating Unicode 4.0.0 (April 2003).  */
    0x03F2,

    0x03F5, 0x1E9B, 0x1FBE,
  };

/* Maximum number of characters that can be the case-folded
   counterparts of a single character, not counting the character
   itself.  This is 1 for towupper, 1 for towlower, and 1 for each
   entry in LONESOME_LOWER.  */
enum
{ CASE_FOLDED_BUFSIZE = 2 + sizeof lonesome_lower / sizeof *lonesome_lower };

/* Find the characters equal to C after case-folding, other than C
   itself, and store them into FOLDED.  Return the number of characters
   stored.  */
static int
case_folded_counterparts (wchar_t c, wchar_t folded[CASE_FOLDED_BUFSIZE])
{
  int i;
  int n = 0;
  wint_t uc = towupper (c);
  wint_t lc = towlower (uc);
  if (uc != c)
    folded[n++] = uc;
  if (lc != uc && lc != c && towupper (lc) == uc)
    folded[n++] = lc;
  for (i = 0; i < sizeof lonesome_lower / sizeof *lonesome_lower; i++)
    {
      wint_t li = lonesome_lower[i];
      if (li != lc && li != uc && li != c && towupper (li) == uc)
        folded[n++] = li;
    }
  return n;
}

typedef int predicate (int);

/* The following list maps the names of the Posix named character classes
   to predicate functions that determine whether a given character is in
   the class.  The leading [ has already been eaten by the lexical
   analyzer.  */
struct dfa_ctype
{
  const char *name;
  predicate *func;
  bool single_byte_only;
};

static const struct dfa_ctype prednames[] = {
  {"alpha", isalpha, false},
  {"upper", isupper, false},
  {"lower", islower, false},
  {"digit", isdigit, true},
  {"xdigit", isxdigit, false},
  {"space", isspace, false},
  {"punct", ispunct, false},
  {"alnum", isalnum, false},
  {"print", isprint, false},
  {"graph", isgraph, false},
  {"cntrl", iscntrl, false},
  {"blank", isblank, false},
  {NULL, NULL, false}
};

static const struct dfa_ctype *_GL_ATTRIBUTE_PURE
find_pred (const char *str)
{
  unsigned int i;
  for (i = 0; prednames[i].name; ++i)
    if (STREQ (str, prednames[i].name))
      break;

  return &prednames[i];
}

/* Multibyte character handling sub-routine for lex.
   Parse a bracket expression and build a struct mb_char_classes.  */
static token
parse_bracket_exp (void)
{
  bool invert;
  int c, c1, c2;
  charclass ccl;

  /* This is a bracket expression that dfaexec is known to
     process correctly.  */
  bool known_bracket_exp = true;

  /* Used to warn about [:space:].
     Bit 0 = first character is a colon.
     Bit 1 = last character is a colon.
     Bit 2 = includes any other character but a colon.
     Bit 3 = includes ranges, char/equiv classes or collation elements.  */
  int colon_warning_state;

  wint_t wc;
  wint_t wc2;
  wint_t wc1 = 0;

  /* Work area to build a mb_char_classes.  */
  struct mb_char_classes *work_mbc;
  size_t chars_al, ranges_al, ch_classes_al, equivs_al, coll_elems_al;

  chars_al = ranges_al = ch_classes_al = equivs_al = coll_elems_al = 0;
  if (dfa->multibyte)
    {
      dfa->mbcsets = maybe_realloc (dfa->mbcsets, dfa->nmbcsets,
                                    &dfa->mbcsets_alloc,
                                    sizeof *dfa->mbcsets);

      /* dfa->multibyte_prop[] hold the index of dfa->mbcsets.
         We will update dfa->multibyte_prop[] in addtok, because we can't
         decide the index in dfa->tokens[].  */

      /* Initialize work area.  */
      work_mbc = &(dfa->mbcsets[dfa->nmbcsets++]);
      memset (work_mbc, 0, sizeof *work_mbc);
    }
  else
    work_mbc = NULL;

  memset (ccl, 0, sizeof ccl);
  FETCH_WC (c, wc, _("unbalanced ["));
  if (c == '^')
    {
      FETCH_WC (c, wc, _("unbalanced ["));
      invert = true;
      known_bracket_exp = using_simple_locale ();
    }
  else
    invert = false;

  colon_warning_state = (c == ':');
  do
    {
      c1 = NOTCHAR;	/* Mark c1 as not initialized.  */
      colon_warning_state &= ~2;

      /* Note that if we're looking at some other [:...:] construct,
         we just treat it as a bunch of ordinary characters.  We can do
         this because we assume regex has checked for syntax errors before
         dfa is ever called.  */
      if (c == '[')
        {
          FETCH_WC (c1, wc1, _("unbalanced ["));

          if ((c1 == ':' && (syntax_bits & RE_CHAR_CLASSES))
              || c1 == '.' || c1 == '=')
            {
              enum { MAX_BRACKET_STRING_LEN = 32 };
              char str[MAX_BRACKET_STRING_LEN + 1];
              size_t len = 0;
              for (;;)
                {
                  FETCH_WC (c, wc, _("unbalanced ["));
                  if ((c == c1 && *lexptr == ']') || lexleft == 0)
                    break;
                  if (len < MAX_BRACKET_STRING_LEN)
                    str[len++] = c;
                  else
                    /* This is in any case an invalid class name.  */
                    str[0] = '\0';
                }
              str[len] = '\0';

              /* Fetch bracket.  */
              FETCH_WC (c, wc, _("unbalanced ["));
              if (c1 == ':')
                /* Build character class.  POSIX allows character
                   classes to match multicharacter collating elements,
                   but the regex code does not support that, so do not
                   worry about that possibility.  */
                {
                  char const *class
                    = (case_fold && (STREQ (str, "upper")
                                     || STREQ (str, "lower")) ? "alpha" : str);
                  const struct dfa_ctype *pred = find_pred (class);
                  if (!pred)
                    dfaerror (_("invalid character class"));

                  if (dfa->multibyte && !pred->single_byte_only)
                    {
                      /* Store the character class as wctype_t.  */
                      wctype_t wt = wctype (class);

                      work_mbc->ch_classes
                        = maybe_realloc (work_mbc->ch_classes,
                                         work_mbc->nch_classes, &ch_classes_al,
                                         sizeof *work_mbc->ch_classes);
                      work_mbc->ch_classes[work_mbc->nch_classes++] = wt;
                    }

                  for (c2 = 0; c2 < NOTCHAR; ++c2)
                    if (pred->func (c2))
                      setbit (c2, ccl);
                }
              else
                known_bracket_exp = false;

              colon_warning_state |= 8;

              /* Fetch new lookahead character.  */
              FETCH_WC (c1, wc1, _("unbalanced ["));
              continue;
            }

          /* We treat '[' as a normal character here.  c/c1/wc/wc1
             are already set up.  */
        }

      if (c == '\\' && (syntax_bits & RE_BACKSLASH_ESCAPE_IN_LISTS))
        FETCH_WC (c, wc, _("unbalanced ["));

      if (c1 == NOTCHAR)
        FETCH_WC (c1, wc1, _("unbalanced ["));

      if (c1 == '-')
        /* build range characters.  */
        {
          FETCH_WC (c2, wc2, _("unbalanced ["));

          /* A bracket expression like [a-[.aa.]] matches an unknown set.
             Treat it like [-a[.aa.]] while parsing it, and
             remember that the set is unknown.  */
          if (c2 == '[' && *lexptr == '.')
            {
              known_bracket_exp = false;
              c2 = ']';
            }

          if (c2 != ']')
            {
              if (c2 == '\\' && (syntax_bits & RE_BACKSLASH_ESCAPE_IN_LISTS))
                FETCH_WC (c2, wc2, _("unbalanced ["));

              if (dfa->multibyte)
                {
                  /* When case folding map a range, say [m-z] (or even [M-z])
                     to the pair of ranges, [m-z] [M-Z].  Although this code
                     is wrong in multiple ways, it's never used in practice.
                     FIXME: Remove this (and related) unused code.  */
                  if (wc != WEOF && wc2 != WEOF)
                    {
                      work_mbc->ranges
                        = maybe_realloc (work_mbc->ranges,
                                         work_mbc->nranges + 2,
                                         &ranges_al, sizeof *work_mbc->ranges);
                      work_mbc->ranges[work_mbc->nranges].beg
                        = case_fold ? towlower (wc) : wc;
                      work_mbc->ranges[work_mbc->nranges++].end
                        = case_fold ? towlower (wc2) : wc2;

                      if (case_fold && (iswalpha (wc) || iswalpha (wc2)))
                        {
                          work_mbc->ranges[work_mbc->nranges].beg
                            = towupper (wc);
                          work_mbc->ranges[work_mbc->nranges++].end
                            = towupper (wc2);
                        }
                    }
                }
              else if (using_simple_locale ())
                {
                  for (c1 = c; c1 <= c2; c1++)
                    setbit (c1, ccl);
                  if (case_fold)
                    {
                      int uc = toupper (c);
                      int uc2 = toupper (c2);
                      for (c1 = 0; c1 < NOTCHAR; c1++)
                        {
                          int uc1 = toupper (c1);
                          if (uc <= uc1 && uc1 <= uc2)
                            setbit (c1, ccl);
                        }
                    }
                }
              else
                known_bracket_exp = false;

              colon_warning_state |= 8;
              FETCH_WC (c1, wc1, _("unbalanced ["));
              continue;
            }

          /* In the case [x-], the - is an ordinary hyphen,
             which is left in c1, the lookahead character.  */
          lexptr -= cur_mb_len;
          lexleft += cur_mb_len;
        }

      colon_warning_state |= (c == ':') ? 2 : 4;

      if (!dfa->multibyte)
        {
          if (case_fold)
            setbit_case_fold_c (c, ccl);
          else
            setbit (c, ccl);
          continue;
        }

      if (wc == WEOF)
        known_bracket_exp = false;
      else
        {
          wchar_t folded[CASE_FOLDED_BUFSIZE + 1];
          int i;
          int n = (case_fold ? case_folded_counterparts (wc, folded + 1) + 1
                   : 1);
          folded[0] = wc;
          for (i = 0; i < n; i++)
            if (!setbit_wc (folded[i], ccl))
              {
                work_mbc->chars
                  = maybe_realloc (work_mbc->chars, work_mbc->nchars,
                                   &chars_al, sizeof *work_mbc->chars);
                work_mbc->chars[work_mbc->nchars++] = folded[i];
              }
        }
    }
  while ((wc = wc1, (c = c1) != ']'));

  if (colon_warning_state == 7)
    dfawarn (_("character class syntax is [[:space:]], not [:space:]"));

  if (! known_bracket_exp)
    return BACKREF;

  if (dfa->multibyte)
    {
      static charclass zeroclass;
      work_mbc->invert = invert;
      work_mbc->cset = equal (ccl, zeroclass) ? -1 : charclass_index (ccl);
      return MBCSET;
    }

  if (invert)
    {
      assert (!dfa->multibyte);
      notset (ccl);
      if (syntax_bits & RE_HAT_LISTS_NOT_NEWLINE)
        clrbit (eolbyte, ccl);
    }

  return CSET + charclass_index (ccl);
}

#define PUSH_LEX_STATE(s)			\
  do						\
    {						\
      char const *lexptr_saved = lexptr;	\
      size_t lexleft_saved = lexleft;		\
      lexptr = (s);				\
      lexleft = strlen (lexptr)

#define POP_LEX_STATE()				\
      lexptr = lexptr_saved;			\
      lexleft = lexleft_saved;			\
    }						\
  while (0)

static token
lex (void)
{
  int c, c2;
  bool backslash = false;
  charclass ccl;
  int i;

  /* Basic plan: We fetch a character.  If it's a backslash,
     we set the backslash flag and go through the loop again.
     On the plus side, this avoids having a duplicate of the
     main switch inside the backslash case.  On the minus side,
     it means that just about every case begins with
     "if (backslash) ...".  */
  for (i = 0; i < 2; ++i)
    {
      FETCH_WC (c, wctok, NULL);

      switch (c)
        {
        case '\\':
          if (backslash)
            goto normal_char;
          if (lexleft == 0)
            dfaerror (_("unfinished \\ escape"));
          backslash = true;
          break;

        case '^':
          if (backslash)
            goto normal_char;
          if (syntax_bits & RE_CONTEXT_INDEP_ANCHORS
              || lasttok == END || lasttok == LPAREN || lasttok == OR)
            return lasttok = BEGLINE;
          goto normal_char;

        case '$':
          if (backslash)
            goto normal_char;
          if (syntax_bits & RE_CONTEXT_INDEP_ANCHORS
              || lexleft == 0
              || (syntax_bits & RE_NO_BK_PARENS
                  ? lexleft > 0 && *lexptr == ')'
                  : lexleft > 1 && lexptr[0] == '\\' && lexptr[1] == ')')
              || (syntax_bits & RE_NO_BK_VBAR
                  ? lexleft > 0 && *lexptr == '|'
                  : lexleft > 1 && lexptr[0] == '\\' && lexptr[1] == '|')
              || ((syntax_bits & RE_NEWLINE_ALT)
                  && lexleft > 0 && *lexptr == '\n'))
            return lasttok = ENDLINE;
          goto normal_char;

        case '1':
        case '2':
        case '3':
        case '4':
        case '5':
        case '6':
        case '7':
        case '8':
        case '9':
          if (backslash && !(syntax_bits & RE_NO_BK_REFS))
            {
              laststart = false;
              return lasttok = BACKREF;
            }
          goto normal_char;

        case '`':
          if (backslash && !(syntax_bits & RE_NO_GNU_OPS))
            return lasttok = BEGLINE; /* FIXME: should be beginning of string */
          goto normal_char;

        case '\'':
          if (backslash && !(syntax_bits & RE_NO_GNU_OPS))
            return lasttok = ENDLINE;   /* FIXME: should be end of string */
          goto normal_char;

        case '<':
          if (backslash && !(syntax_bits & RE_NO_GNU_OPS))
            return lasttok = BEGWORD;
          goto normal_char;

        case '>':
          if (backslash && !(syntax_bits & RE_NO_GNU_OPS))
            return lasttok = ENDWORD;
          goto normal_char;

        case 'b':
          if (backslash && !(syntax_bits & RE_NO_GNU_OPS))
            return lasttok = LIMWORD;
          goto normal_char;

        case 'B':
          if (backslash && !(syntax_bits & RE_NO_GNU_OPS))
            return lasttok = NOTLIMWORD;
          goto normal_char;

        case '?':
          if (syntax_bits & RE_LIMITED_OPS)
            goto normal_char;
          if (backslash != ((syntax_bits & RE_BK_PLUS_QM) != 0))
            goto normal_char;
          if (!(syntax_bits & RE_CONTEXT_INDEP_OPS) && laststart)
            goto normal_char;
          return lasttok = QMARK;

        case '*':
          if (backslash)
            goto normal_char;
          if (!(syntax_bits & RE_CONTEXT_INDEP_OPS) && laststart)
            goto normal_char;
          return lasttok = STAR;

        case '+':
          if (syntax_bits & RE_LIMITED_OPS)
            goto normal_char;
          if (backslash != ((syntax_bits & RE_BK_PLUS_QM) != 0))
            goto normal_char;
          if (!(syntax_bits & RE_CONTEXT_INDEP_OPS) && laststart)
            goto normal_char;
          return lasttok = PLUS;

        case '{':
          if (!(syntax_bits & RE_INTERVALS))
            goto normal_char;
          if (backslash != ((syntax_bits & RE_NO_BK_BRACES) == 0))
            goto normal_char;
          if (!(syntax_bits & RE_CONTEXT_INDEP_OPS) && laststart)
            goto normal_char;

          /* Cases:
             {M} - exact count
             {M,} - minimum count, maximum is infinity
             {,N} - 0 through N
             {,} - 0 to infinity (same as '*')
             {M,N} - M through N */
          {
            char const *p = lexptr;
            char const *lim = p + lexleft;
            minrep = maxrep = -1;
            for (; p != lim && ISASCIIDIGIT (*p); p++)
              {
                if (minrep < 0)
                  minrep = *p - '0';
                else
                  minrep = MIN (RE_DUP_MAX + 1, minrep * 10 + *p - '0');
              }
            if (p != lim)
              {
                if (*p != ',')
                  maxrep = minrep;
                else
                  {
                    if (minrep < 0)
                      minrep = 0;
                    while (++p != lim && ISASCIIDIGIT (*p))
                      {
                        if (maxrep < 0)
                          maxrep = *p - '0';
                        else
                          maxrep = MIN (RE_DUP_MAX + 1, maxrep * 10 + *p - '0');
                      }
                  }
              }
            if (! ((! backslash || (p != lim && *p++ == '\\'))
                   && p != lim && *p++ == '}'
                   && 0 <= minrep && (maxrep < 0 || minrep <= maxrep)))
              {
                if (syntax_bits & RE_INVALID_INTERVAL_ORD)
                  goto normal_char;
                dfaerror (_("invalid content of \\{\\}"));
              }
            if (RE_DUP_MAX < maxrep)
              dfaerror (_("regular expression too big"));
            lexptr = p;
            lexleft = lim - p;
          }
          laststart = false;
          return lasttok = REPMN;

        case '|':
          if (syntax_bits & RE_LIMITED_OPS)
            goto normal_char;
          if (backslash != ((syntax_bits & RE_NO_BK_VBAR) == 0))
            goto normal_char;
          laststart = true;
          return lasttok = OR;

        case '\n':
          if (syntax_bits & RE_LIMITED_OPS
              || backslash || !(syntax_bits & RE_NEWLINE_ALT))
            goto normal_char;
          laststart = true;
          return lasttok = OR;

        case '(':
          if (backslash != ((syntax_bits & RE_NO_BK_PARENS) == 0))
            goto normal_char;
          ++parens;
          laststart = true;
          return lasttok = LPAREN;

        case ')':
          if (backslash != ((syntax_bits & RE_NO_BK_PARENS) == 0))
            goto normal_char;
          if (parens == 0 && syntax_bits & RE_UNMATCHED_RIGHT_PAREN_ORD)
            goto normal_char;
          --parens;
          laststart = false;
          return lasttok = RPAREN;

        case '.':
          if (backslash)
            goto normal_char;
          if (dfa->multibyte)
            {
              /* In multibyte environment period must match with a single
                 character not a byte.  So we use ANYCHAR.  */
              laststart = false;
              return lasttok = ANYCHAR;
            }
          zeroset (ccl);
          notset (ccl);
          if (!(syntax_bits & RE_DOT_NEWLINE))
            clrbit (eolbyte, ccl);
          if (syntax_bits & RE_DOT_NOT_NULL)
            clrbit ('\0', ccl);
          laststart = false;
          return lasttok = CSET + charclass_index (ccl);

        case 's':
        case 'S':
          if (!backslash || (syntax_bits & RE_NO_GNU_OPS))
            goto normal_char;
          if (!dfa->multibyte)
            {
              zeroset (ccl);
              for (c2 = 0; c2 < NOTCHAR; ++c2)
                if (isspace (c2))
                  setbit (c2, ccl);
              if (c == 'S')
                notset (ccl);
              laststart = false;
              return lasttok = CSET + charclass_index (ccl);
            }

          /* FIXME: see if optimizing this, as is done with ANYCHAR and
             add_utf8_anychar, makes sense.  */

          /* \s and \S are documented to be equivalent to [[:space:]] and
             [^[:space:]] respectively, so tell the lexer to process those
             strings, each minus its "already processed" '['.  */
          PUSH_LEX_STATE (c == 's' ? "[:space:]]" : "^[:space:]]");

          lasttok = parse_bracket_exp ();

          POP_LEX_STATE ();

          laststart = false;
          return lasttok;

        case 'w':
        case 'W':
          if (!backslash || (syntax_bits & RE_NO_GNU_OPS))
            goto normal_char;

          if (!dfa->multibyte)
            {
              zeroset (ccl);
              for (c2 = 0; c2 < NOTCHAR; ++c2)
                if (IS_WORD_CONSTITUENT (c2))
                  setbit (c2, ccl);
              if (c == 'W')
                notset (ccl);
              laststart = false;
              return lasttok = CSET + charclass_index (ccl);
            }

          /* FIXME: see if optimizing this, as is done with ANYCHAR and
             add_utf8_anychar, makes sense.  */

          /* \w and \W are documented to be equivalent to [_[:alnum:]] and
             [^_[:alnum:]] respectively, so tell the lexer to process those
             strings, each minus its "already processed" '['.  */
          PUSH_LEX_STATE (c == 'w' ? "_[:alnum:]]" : "^_[:alnum:]]");

          lasttok = parse_bracket_exp ();

          POP_LEX_STATE ();

          laststart = false;
          return lasttok;

        case '[':
          if (backslash)
            goto normal_char;
          laststart = false;
          return lasttok = parse_bracket_exp ();

        default:
        normal_char:
          laststart = false;
          /* For multibyte character sets, folding is done in atom.  Always
             return WCHAR.  */
          if (dfa->multibyte)
            return lasttok = WCHAR;

          if (case_fold && isalpha (c))
            {
              zeroset (ccl);
              setbit_case_fold_c (c, ccl);
              return lasttok = CSET + charclass_index (ccl);
            }

          return lasttok = c;
        }
    }

  /* The above loop should consume at most a backslash
     and some other character.  */
  abort ();
  return END;                   /* keeps pedantic compilers happy.  */
}

/* Recursive descent parser for regular expressions.  */

static token tok;               /* Lookahead token.  */
static size_t depth;            /* Current depth of a hypothetical stack
                                   holding deferred productions.  This is
                                   used to determine the depth that will be
                                   required of the real stack later on in
                                   dfaanalyze.  */

static void
addtok_mb (token t, int mbprop)
{
  if (dfa->talloc == dfa->tindex)
    {
      dfa->tokens = x2nrealloc (dfa->tokens, &dfa->talloc,
                                sizeof *dfa->tokens);
      if (dfa->multibyte)
        dfa->multibyte_prop = xnrealloc (dfa->multibyte_prop, dfa->talloc,
                                         sizeof *dfa->multibyte_prop);
    }
  if (dfa->multibyte)
    dfa->multibyte_prop[dfa->tindex] = mbprop;
  dfa->tokens[dfa->tindex++] = t;

  switch (t)
    {
    case QMARK:
    case STAR:
    case PLUS:
      break;

    case CAT:
    case OR:
      --depth;
      break;

    case BACKREF:
      dfa->fast = false;
      /* fallthrough */
    default:
      ++dfa->nleaves;
      /* fallthrough */
    case EMPTY:
      ++depth;
      break;
    }
  if (depth > dfa->depth)
    dfa->depth = depth;
}

static void addtok_wc (wint_t wc);

/* Add the given token to the parse tree, maintaining the depth count and
   updating the maximum depth if necessary.  */
static void
addtok (token t)
{
  if (dfa->multibyte && t == MBCSET)
    {
      bool need_or = false;
      struct mb_char_classes *work_mbc = &dfa->mbcsets[dfa->nmbcsets - 1];

      /* Extract wide characters into alternations for better performance.
         This does not require UTF-8.  */
      if (!work_mbc->invert)
        {
          size_t i;
          for (i = 0; i < work_mbc->nchars; i++)
            {
              addtok_wc (work_mbc->chars[i]);
              if (need_or)
                addtok (OR);
              need_or = true;
            }
          work_mbc->nchars = 0;
        }

      /* If the MBCSET is non-inverted and doesn't include neither
         character classes including multibyte characters, range
         expressions, equivalence classes nor collating elements,
         it can be replaced to a simple CSET. */
      if (work_mbc->invert
          || work_mbc->nch_classes != 0
          || work_mbc->nranges != 0
          || work_mbc->nequivs != 0 || work_mbc->ncoll_elems != 0)
        {
          addtok_mb (MBCSET, ((dfa->nmbcsets - 1) << 2) + 3);
          if (need_or)
            addtok (OR);
        }
      else
        {
          /* Characters have been handled above, so it is possible
             that the mbcset is empty now.  Do nothing in that case.  */
          if (work_mbc->cset != -1)
            {
              addtok (CSET + work_mbc->cset);
              if (need_or)
                addtok (OR);
            }
        }
    }
  else
    {
      addtok_mb (t, 3);
    }
}

/* We treat a multibyte character as a single atom, so that DFA
   can treat a multibyte character as a single expression.

   e.g., we construct the following tree from "<mb1><mb2>".
   <mb1(1st-byte)><mb1(2nd-byte)><CAT><mb1(3rd-byte)><CAT>
   <mb2(1st-byte)><mb2(2nd-byte)><CAT><mb2(3rd-byte)><CAT><CAT> */
static void
addtok_wc (wint_t wc)
{
  unsigned char buf[MB_LEN_MAX];
  mbstate_t s = { 0 };
  int i;
  size_t stored_bytes = wcrtomb ((char *) buf, wc, &s);

  if (stored_bytes != (size_t) -1)
    cur_mb_len = stored_bytes;
  else
    {
      /* This is merely stop-gap.  buf[0] is undefined, yet skipping
         the addtok_mb call altogether can corrupt the heap.  */
      cur_mb_len = 1;
      buf[0] = 0;
    }

  addtok_mb (buf[0], cur_mb_len == 1 ? 3 : 1);
  for (i = 1; i < cur_mb_len; i++)
    {
      addtok_mb (buf[i], i == cur_mb_len - 1 ? 2 : 0);
      addtok (CAT);
    }
}

static void
add_utf8_anychar (void)
{
  static const charclass utf8_classes[5] = {
    /* 80-bf: non-leading bytes.  */
    {0, 0, 0, 0, CHARCLASS_WORD_MASK, CHARCLASS_WORD_MASK, 0, 0},

    /* 00-7f: 1-byte sequence.  */
    {CHARCLASS_WORD_MASK, CHARCLASS_WORD_MASK, CHARCLASS_WORD_MASK,
     CHARCLASS_WORD_MASK, 0, 0, 0, 0},

    /* c2-df: 2-byte sequence.  */
    {0, 0, 0, 0, 0, 0, ~3 & CHARCLASS_WORD_MASK, 0},

    /* e0-ef: 3-byte sequence.  */
    {0, 0, 0, 0, 0, 0, 0, 0xffff},

    /* f0-f7: 4-byte sequence.  */
    {0, 0, 0, 0, 0, 0, 0, 0xff0000}
  };
  const unsigned int n = sizeof (utf8_classes) / sizeof (utf8_classes[0]);
  unsigned int i;

  /* Define the five character classes that are needed below.  */
  if (dfa->utf8_anychar_classes[0] == 0)
    for (i = 0; i < n; i++)
      {
        charclass c;
        copyset (utf8_classes[i], c);
        if (i == 1)
          {
            if (!(syntax_bits & RE_DOT_NEWLINE))
              clrbit (eolbyte, c);
            if (syntax_bits & RE_DOT_NOT_NULL)
              clrbit ('\0', c);
          }
        dfa->utf8_anychar_classes[i] = CSET + charclass_index (c);
      }

  /* A valid UTF-8 character is

     ([0x00-0x7f]
     |[0xc2-0xdf][0x80-0xbf]
     |[0xe0-0xef[0x80-0xbf][0x80-0xbf]
     |[0xf0-f7][0x80-0xbf][0x80-0xbf][0x80-0xbf])

     which I'll write more concisely "B|CA|DAA|EAAA".  Factor the [0x00-0x7f]
     and you get "B|(C|(D|EA)A)A".  And since the token buffer is in reverse
     Polish notation, you get "B C D E A CAT OR A CAT OR A CAT OR".  */
  for (i = 1; i < n; i++)
    addtok (dfa->utf8_anychar_classes[i]);
  while (--i > 1)
    {
      addtok (dfa->utf8_anychar_classes[0]);
      addtok (CAT);
      addtok (OR);
    }
}

/* The grammar understood by the parser is as follows.

   regexp:
     regexp OR branch
     branch

   branch:
     branch closure
     closure

   closure:
     closure QMARK
     closure STAR
     closure PLUS
     closure REPMN
     atom

   atom:
     <normal character>
     <multibyte character>
     ANYCHAR
     MBCSET
     CSET
     BACKREF
     BEGLINE
     ENDLINE
     BEGWORD
     ENDWORD
     LIMWORD
     NOTLIMWORD
     LPAREN regexp RPAREN
     <empty>

   The parser builds a parse tree in postfix form in an array of tokens.  */

static void
atom (void)
{
  if (tok == WCHAR)
    {
      if (wctok == WEOF)
        addtok (BACKREF);
      else
        {
          addtok_wc (wctok);

          if (case_fold)
            {
              wchar_t folded[CASE_FOLDED_BUFSIZE];
              int i, n = case_folded_counterparts (wctok, folded);
              for (i = 0; i < n; i++)
                {
                  addtok_wc (folded[i]);
                  addtok (OR);
                }
            }
        }

      tok = lex ();
    }
  else if (tok == ANYCHAR && using_utf8 ())
    {
      /* For UTF-8 expand the period to a series of CSETs that define a valid
         UTF-8 character.  This avoids using the slow multibyte path.  I'm
         pretty sure it would be both profitable and correct to do it for
         any encoding; however, the optimization must be done manually as
         it is done above in add_utf8_anychar.  So, let's start with
         UTF-8: it is the most used, and the structure of the encoding
         makes the correctness more obvious.  */
      add_utf8_anychar ();
      tok = lex ();
    }
  else if ((tok >= 0 && tok < NOTCHAR) || tok >= CSET || tok == BACKREF
           || tok == BEGLINE || tok == ENDLINE || tok == BEGWORD
           || tok == ANYCHAR || tok == MBCSET
           || tok == ENDWORD || tok == LIMWORD || tok == NOTLIMWORD)
    {
      addtok (tok);
      tok = lex ();
    }
  else if (tok == LPAREN)
    {
      tok = lex ();
      regexp ();
      if (tok != RPAREN)
        dfaerror (_("unbalanced ("));
      tok = lex ();
    }
  else
    addtok (EMPTY);
}

/* Return the number of tokens in the given subexpression.  */
static size_t _GL_ATTRIBUTE_PURE
nsubtoks (size_t tindex)
{
  size_t ntoks1;

  switch (dfa->tokens[tindex - 1])
    {
    default:
      return 1;
    case QMARK:
    case STAR:
    case PLUS:
      return 1 + nsubtoks (tindex - 1);
    case CAT:
    case OR:
      ntoks1 = nsubtoks (tindex - 1);
      return 1 + ntoks1 + nsubtoks (tindex - 1 - ntoks1);
    }
}

/* Copy the given subexpression to the top of the tree.  */
static void
copytoks (size_t tindex, size_t ntokens)
{
  size_t i;

  if (dfa->multibyte)
    for (i = 0; i < ntokens; ++i)
      addtok_mb (dfa->tokens[tindex + i], dfa->multibyte_prop[tindex + i]);
  else
    for (i = 0; i < ntokens; ++i)
      addtok_mb (dfa->tokens[tindex + i], 3);
}

static void
closure (void)
{
  int i;
  size_t tindex, ntokens;

  atom ();
  while (tok == QMARK || tok == STAR || tok == PLUS || tok == REPMN)
    if (tok == REPMN && (minrep || maxrep))
      {
        ntokens = nsubtoks (dfa->tindex);
        tindex = dfa->tindex - ntokens;
        if (maxrep < 0)
          addtok (PLUS);
        if (minrep == 0)
          addtok (QMARK);
        for (i = 1; i < minrep; ++i)
          {
            copytoks (tindex, ntokens);
            addtok (CAT);
          }
        for (; i < maxrep; ++i)
          {
            copytoks (tindex, ntokens);
            addtok (QMARK);
            addtok (CAT);
          }
        tok = lex ();
      }
    else if (tok == REPMN)
      {
        dfa->tindex -= nsubtoks (dfa->tindex);
        tok = lex ();
        closure ();
      }
    else
      {
        addtok (tok);
        tok = lex ();
      }
}

static void
branch (void)
{
  closure ();
  while (tok != RPAREN && tok != OR && tok >= 0)
    {
      closure ();
      addtok (CAT);
    }
}

static void
regexp (void)
{
  branch ();
  while (tok == OR)
    {
      tok = lex ();
      branch ();
      addtok (OR);
    }
}

/* Main entry point for the parser.  S is a string to be parsed, len is the
   length of the string, so s can include NUL characters.  D is a pointer to
   the struct dfa to parse into.  */
void
dfaparse (char const *s, size_t len, struct dfa *d)
{
  dfa = d;
  lexptr = s;
  lexleft = len;
  lasttok = END;
  laststart = true;
  parens = 0;
  if (dfa->multibyte)
    {
      cur_mb_len = 0;
      memset (&d->mbs, 0, sizeof d->mbs);
    }

  if (!syntax_bits_set)
    dfaerror (_("no syntax specified"));

  tok = lex ();
  depth = d->depth;

  regexp ();

  if (tok != END)
    dfaerror (_("unbalanced )"));

  addtok (END - d->nregexps);
  addtok (CAT);

  if (d->nregexps)
    addtok (OR);

  ++d->nregexps;
}

/* Some primitives for operating on sets of positions.  */

/* Copy one set to another.  */
static void
copy (position_set const *src, position_set * dst)
{
  if (dst->alloc < src->nelem)
    {
      free (dst->elems);
      dst->alloc = src->nelem;
      dst->elems = x2nrealloc (NULL, &dst->alloc, sizeof *dst->elems);
    }
  memcpy (dst->elems, src->elems, src->nelem * sizeof *dst->elems);
  dst->nelem = src->nelem;
}

static void
alloc_position_set (position_set * s, size_t size)
{
  s->elems = xnmalloc (size, sizeof *s->elems);
  s->alloc = size;
  s->nelem = 0;
}

/* Insert position P in set S.  S is maintained in sorted order on
   decreasing index.  If there is already an entry in S with P.index
   then merge (logically-OR) P's constraints into the one in S.
   S->elems must point to an array large enough to hold the resulting set.  */
static void
insert (position p, position_set * s)
{
  size_t count = s->nelem;
  size_t lo = 0, hi = count;
  size_t i;
  while (lo < hi)
    {
      size_t mid = (lo + hi) >> 1;
      if (s->elems[mid].index > p.index)
        lo = mid + 1;
      else
        hi = mid;
    }

  if (lo < count && p.index == s->elems[lo].index)
    {
      s->elems[lo].constraint |= p.constraint;
      return;
    }

  s->elems = maybe_realloc (s->elems, count, &s->alloc, sizeof *s->elems);
  for (i = count; i > lo; i--)
    s->elems[i] = s->elems[i - 1];
  s->elems[lo] = p;
  ++s->nelem;
}

/* Merge two sets of positions into a third.  The result is exactly as if
   the positions of both sets were inserted into an initially empty set.  */
static void
merge (position_set const *s1, position_set const *s2, position_set * m)
{
  size_t i = 0, j = 0;

  if (m->alloc < s1->nelem + s2->nelem)
    {
      free (m->elems);
      m->elems = maybe_realloc (NULL, s1->nelem + s2->nelem, &m->alloc,
                                sizeof *m->elems);
    }
  m->nelem = 0;
  while (i < s1->nelem && j < s2->nelem)
    if (s1->elems[i].index > s2->elems[j].index)
      m->elems[m->nelem++] = s1->elems[i++];
    else if (s1->elems[i].index < s2->elems[j].index)
      m->elems[m->nelem++] = s2->elems[j++];
    else
      {
        m->elems[m->nelem] = s1->elems[i++];
        m->elems[m->nelem++].constraint |= s2->elems[j++].constraint;
      }
  while (i < s1->nelem)
    m->elems[m->nelem++] = s1->elems[i++];
  while (j < s2->nelem)
    m->elems[m->nelem++] = s2->elems[j++];
}

/* Delete a position from a set.  */
static void
delete (position p, position_set * s)
{
  size_t i;

  for (i = 0; i < s->nelem; ++i)
    if (p.index == s->elems[i].index)
      break;
  if (i < s->nelem)
    for (--s->nelem; i < s->nelem; ++i)
      s->elems[i] = s->elems[i + 1];
}

/* Find the index of the state corresponding to the given position set with
   the given preceding context, or create a new state if there is no such
   state.  Context tells whether we got here on a newline or letter.  */
static state_num
state_index (struct dfa *d, position_set const *s, int context)
{
  size_t hash = 0;
  int constraint;
  state_num i, j;

  for (i = 0; i < s->nelem; ++i)
    hash ^= s->elems[i].index + s->elems[i].constraint;

  /* Try to find a state that exactly matches the proposed one.  */
  for (i = 0; i < d->sindex; ++i)
    {
      if (hash != d->states[i].hash || s->nelem != d->states[i].elems.nelem
          || context != d->states[i].context)
        continue;
      for (j = 0; j < s->nelem; ++j)
        if (s->elems[j].constraint
            != d->states[i].elems.elems[j].constraint
            || s->elems[j].index != d->states[i].elems.elems[j].index)
          break;
      if (j == s->nelem)
        return i;
    }

  /* We'll have to create a new state.  */
  d->states = maybe_realloc (d->states, d->sindex, &d->salloc,
                             sizeof *d->states);
  d->states[i].hash = hash;
  alloc_position_set (&d->states[i].elems, s->nelem);
  copy (s, &d->states[i].elems);
  d->states[i].context = context;
  d->states[i].has_backref = false;
  d->states[i].has_mbcset = false;
  d->states[i].constraint = 0;
  d->states[i].first_end = 0;
  d->states[i].mbps.nelem = 0;
  d->states[i].mbps.elems = NULL;

  for (j = 0; j < s->nelem; ++j)
    if (d->tokens[s->elems[j].index] < 0)
      {
        constraint = s->elems[j].constraint;
        if (SUCCEEDS_IN_CONTEXT (constraint, context, CTX_ANY))
          d->states[i].constraint |= constraint;
        if (!d->states[i].first_end)
          d->states[i].first_end = d->tokens[s->elems[j].index];
      }
    else if (d->tokens[s->elems[j].index] == BACKREF)
      {
        d->states[i].constraint = NO_CONSTRAINT;
        d->states[i].has_backref = true;
      }

  ++d->sindex;

  return i;
}

/* Find the epsilon closure of a set of positions.  If any position of the set
   contains a symbol that matches the empty string in some context, replace
   that position with the elements of its follow labeled with an appropriate
   constraint.  Repeat exhaustively until no funny positions are left.
   S->elems must be large enough to hold the result.  */
static void
epsclosure (position_set *s, struct dfa const *d, char *visited)
{
  size_t i, j;
  position p, old;
  bool initialized = false;

  for (i = 0; i < s->nelem; ++i)
    if (d->tokens[s->elems[i].index] >= NOTCHAR
        && d->tokens[s->elems[i].index] != BACKREF
        && d->tokens[s->elems[i].index] != ANYCHAR
        && d->tokens[s->elems[i].index] != MBCSET
        && d->tokens[s->elems[i].index] < CSET)
      {
        if (!initialized)
          {
            memset (visited, 0, d->tindex * sizeof (*visited));
            initialized = true;
          }
        old = s->elems[i];
        p.constraint = old.constraint;
        delete (s->elems[i], s);
        if (visited[old.index])
          {
            --i;
            continue;
          }
        visited[old.index] = 1;
        switch (d->tokens[old.index])
          {
          case BEGLINE:
            p.constraint &= BEGLINE_CONSTRAINT;
            break;
          case ENDLINE:
            p.constraint &= ENDLINE_CONSTRAINT;
            break;
          case BEGWORD:
            p.constraint &= BEGWORD_CONSTRAINT;
            break;
          case ENDWORD:
            p.constraint &= ENDWORD_CONSTRAINT;
            break;
          case LIMWORD:
            p.constraint &= LIMWORD_CONSTRAINT;
            break;
          case NOTLIMWORD:
            p.constraint &= NOTLIMWORD_CONSTRAINT;
            break;
          default:
            break;
          }
        for (j = 0; j < d->follows[old.index].nelem; ++j)
          {
            p.index = d->follows[old.index].elems[j].index;
            insert (p, s);
          }
        /* Force rescan to start at the beginning.  */
        i = -1;
      }
}

/* Returns the set of contexts for which there is at least one
   character included in C.  */

static int
charclass_context (charclass c)
{
  int context = 0;
  unsigned int j;

  if (tstbit (eolbyte, c))
    context |= CTX_NEWLINE;

  for (j = 0; j < CHARCLASS_WORDS; ++j)
    {
      if (c[j] & letters[j])
        context |= CTX_LETTER;
      if (c[j] & ~(letters[j] | newline[j]))
        context |= CTX_NONE;
    }

  return context;
}

/* Returns the contexts on which the position set S depends.  Each context
   in the set of returned contexts (let's call it SC) may have a different
   follow set than other contexts in SC, and also different from the
   follow set of the complement set (sc ^ CTX_ANY).  However, all contexts
   in the complement set will have the same follow set.  */

static int _GL_ATTRIBUTE_PURE
state_separate_contexts (position_set const *s)
{
  int separate_contexts = 0;
  size_t j;

  for (j = 0; j < s->nelem; ++j)
    {
      if (PREV_NEWLINE_DEPENDENT (s->elems[j].constraint))
        separate_contexts |= CTX_NEWLINE;
      if (PREV_LETTER_DEPENDENT (s->elems[j].constraint))
        separate_contexts |= CTX_LETTER;
    }

  return separate_contexts;
}


/* Perform bottom-up analysis on the parse tree, computing various functions.
   Note that at this point, we're pretending constructs like \< are real
   characters rather than constraints on what can follow them.

   Nullable:  A node is nullable if it is at the root of a regexp that can
   match the empty string.
   *  EMPTY leaves are nullable.
   * No other leaf is nullable.
   * A QMARK or STAR node is nullable.
   * A PLUS node is nullable if its argument is nullable.
   * A CAT node is nullable if both its arguments are nullable.
   * An OR node is nullable if either argument is nullable.

   Firstpos:  The firstpos of a node is the set of positions (nonempty leaves)
   that could correspond to the first character of a string matching the
   regexp rooted at the given node.
   * EMPTY leaves have empty firstpos.
   * The firstpos of a nonempty leaf is that leaf itself.
   * The firstpos of a QMARK, STAR, or PLUS node is the firstpos of its
     argument.
   * The firstpos of a CAT node is the firstpos of the left argument, union
     the firstpos of the right if the left argument is nullable.
   * The firstpos of an OR node is the union of firstpos of each argument.

   Lastpos:  The lastpos of a node is the set of positions that could
   correspond to the last character of a string matching the regexp at
   the given node.
   * EMPTY leaves have empty lastpos.
   * The lastpos of a nonempty leaf is that leaf itself.
   * The lastpos of a QMARK, STAR, or PLUS node is the lastpos of its
     argument.
   * The lastpos of a CAT node is the lastpos of its right argument, union
     the lastpos of the left if the right argument is nullable.
   * The lastpos of an OR node is the union of the lastpos of each argument.

   Follow:  The follow of a position is the set of positions that could
   correspond to the character following a character matching the node in
   a string matching the regexp.  At this point we consider special symbols
   that match the empty string in some context to be just normal characters.
   Later, if we find that a special symbol is in a follow set, we will
   replace it with the elements of its follow, labeled with an appropriate
   constraint.
   * Every node in the firstpos of the argument of a STAR or PLUS node is in
     the follow of every node in the lastpos.
   * Every node in the firstpos of the second argument of a CAT node is in
     the follow of every node in the lastpos of the first argument.

   Because of the postfix representation of the parse tree, the depth-first
   analysis is conveniently done by a linear scan with the aid of a stack.
   Sets are stored as arrays of the elements, obeying a stack-like allocation
   scheme; the number of elements in each set deeper in the stack can be
   used to determine the address of a particular set's array.  */
void
dfaanalyze (struct dfa *d, int searchflag)
{
  /* Array allocated to hold position sets.  */
  position *posalloc = xnmalloc (d->nleaves, 2 * sizeof *posalloc);
  /* Firstpos and lastpos elements.  */
  position *firstpos = posalloc + d->nleaves;
  position *lastpos = firstpos + d->nleaves;

  /* Stack for element counts and nullable flags.  */
  struct
  {
    /* Whether the entry is nullable.  */
    bool nullable;

    /* Counts of firstpos and lastpos sets.  */
    size_t nfirstpos;
    size_t nlastpos;
  } *stkalloc = xnmalloc (d->depth, sizeof *stkalloc), *stk = stkalloc;

  position_set tmp;             /* Temporary set for merging sets.  */
  position_set merged;          /* Result of merging sets.  */
  int separate_contexts;        /* Context wanted by some position.  */
  size_t i, j;
  position *pos;
  char *visited = xnmalloc (d->tindex, sizeof *visited);

#ifdef DEBUG
  fprintf (stderr, "dfaanalyze:\n");
  for (i = 0; i < d->tindex; ++i)
    {
      fprintf (stderr, " %zd:", i);
      prtok (d->tokens[i]);
    }
  putc ('\n', stderr);
#endif

  d->searchflag = searchflag != 0;
  alloc_position_set (&merged, d->nleaves);
  d->follows = xcalloc (d->tindex, sizeof *d->follows);

  for (i = 0; i < d->tindex; ++i)
    {
      switch (d->tokens[i])
        {
        case EMPTY:
          /* The empty set is nullable.  */
          stk->nullable = true;

          /* The firstpos and lastpos of the empty leaf are both empty.  */
          stk->nfirstpos = stk->nlastpos = 0;
          stk++;
          break;

        case STAR:
        case PLUS:
          /* Every element in the firstpos of the argument is in the follow
             of every element in the lastpos.  */
          tmp.nelem = stk[-1].nfirstpos;
          tmp.elems = firstpos;
          pos = lastpos;
          for (j = 0; j < stk[-1].nlastpos; ++j)
            {
              merge (&tmp, &d->follows[pos[j].index], &merged);
              copy (&merged, &d->follows[pos[j].index]);
            }
          /* fallthrough */

        case QMARK:
          /* A QMARK or STAR node is automatically nullable.  */
          if (d->tokens[i] != PLUS)
            stk[-1].nullable = true;
          break;

        case CAT:
          /* Every element in the firstpos of the second argument is in the
             follow of every element in the lastpos of the first argument.  */
          tmp.nelem = stk[-1].nfirstpos;
          tmp.elems = firstpos;
          pos = lastpos + stk[-1].nlastpos;
          for (j = 0; j < stk[-2].nlastpos; ++j)
            {
              merge (&tmp, &d->follows[pos[j].index], &merged);
              copy (&merged, &d->follows[pos[j].index]);
            }

          /* The firstpos of a CAT node is the firstpos of the first argument,
             union that of the second argument if the first is nullable.  */
          if (stk[-2].nullable)
            stk[-2].nfirstpos += stk[-1].nfirstpos;
          else
            firstpos += stk[-1].nfirstpos;

          /* The lastpos of a CAT node is the lastpos of the second argument,
             union that of the first argument if the second is nullable.  */
          if (stk[-1].nullable)
            stk[-2].nlastpos += stk[-1].nlastpos;
          else
            {
              pos = lastpos + stk[-2].nlastpos;
              for (j = stk[-1].nlastpos; j-- > 0;)
                pos[j] = lastpos[j];
              lastpos += stk[-2].nlastpos;
              stk[-2].nlastpos = stk[-1].nlastpos;
            }

          /* A CAT node is nullable if both arguments are nullable.  */
          stk[-2].nullable &= stk[-1].nullable;
          stk--;
          break;

        case OR:
          /* The firstpos is the union of the firstpos of each argument.  */
          stk[-2].nfirstpos += stk[-1].nfirstpos;

          /* The lastpos is the union of the lastpos of each argument.  */
          stk[-2].nlastpos += stk[-1].nlastpos;

          /* An OR node is nullable if either argument is nullable.  */
          stk[-2].nullable |= stk[-1].nullable;
          stk--;
          break;

        default:
          /* Anything else is a nonempty position.  (Note that special
             constructs like \< are treated as nonempty strings here;
             an "epsilon closure" effectively makes them nullable later.
             Backreferences have to get a real position so we can detect
             transitions on them later.  But they are nullable.  */
          stk->nullable = d->tokens[i] == BACKREF;

          /* This position is in its own firstpos and lastpos.  */
          stk->nfirstpos = stk->nlastpos = 1;
          stk++;

          --firstpos, --lastpos;
          firstpos->index = lastpos->index = i;
          firstpos->constraint = lastpos->constraint = NO_CONSTRAINT;

          /* Allocate the follow set for this position.  */
          alloc_position_set (&d->follows[i], 1);
          break;
        }
#ifdef DEBUG
      /* ... balance the above nonsyntactic #ifdef goo...  */
      fprintf (stderr, "node %zd:", i);
      prtok (d->tokens[i]);
      putc ('\n', stderr);
      fprintf (stderr,
               stk[-1].nullable ? " nullable: yes\n" : " nullable: no\n");
      fprintf (stderr, " firstpos:");
      for (j = stk[-1].nfirstpos; j-- > 0;)
        {
          fprintf (stderr, " %zd:", firstpos[j].index);
          prtok (d->tokens[firstpos[j].index]);
        }
      fprintf (stderr, "\n lastpos:");
      for (j = stk[-1].nlastpos; j-- > 0;)
        {
          fprintf (stderr, " %zd:", lastpos[j].index);
          prtok (d->tokens[lastpos[j].index]);
        }
      putc ('\n', stderr);
#endif
    }

  /* For each follow set that is the follow set of a real position, replace
     it with its epsilon closure.  */
  for (i = 0; i < d->tindex; ++i)
    if (d->tokens[i] < NOTCHAR || d->tokens[i] == BACKREF
        || d->tokens[i] == ANYCHAR || d->tokens[i] == MBCSET
        || d->tokens[i] >= CSET)
      {
#ifdef DEBUG
        fprintf (stderr, "follows(%zd:", i);
        prtok (d->tokens[i]);
        fprintf (stderr, "):");
        for (j = d->follows[i].nelem; j-- > 0;)
          {
            fprintf (stderr, " %zd:", d->follows[i].elems[j].index);
            prtok (d->tokens[d->follows[i].elems[j].index]);
          }
        putc ('\n', stderr);
#endif
        copy (&d->follows[i], &merged);
        epsclosure (&merged, d, visited);
        copy (&merged, &d->follows[i]);
      }

  /* Get the epsilon closure of the firstpos of the regexp.  The result will
     be the set of positions of state 0.  */
  merged.nelem = 0;
  for (i = 0; i < stk[-1].nfirstpos; ++i)
    insert (firstpos[i], &merged);
  epsclosure (&merged, d, visited);

  /* Build the initial state.  */
  separate_contexts = state_separate_contexts (&merged);
  if (separate_contexts & CTX_NEWLINE)
    state_index (d, &merged, CTX_NEWLINE);
  d->initstate_others = d->min_trcount
    = state_index (d, &merged, separate_contexts ^ CTX_ANY);
  if (separate_contexts & CTX_LETTER)
    d->initstate_letter = d->min_trcount
      = state_index (d, &merged, CTX_LETTER);
  else
    d->initstate_letter = d->initstate_others;
  d->min_trcount++;

  free (posalloc);
  free (stkalloc);
  free (merged.elems);
  free (visited);
}


/* Find, for each character, the transition out of state s of d, and store
   it in the appropriate slot of trans.

   We divide the positions of s into groups (positions can appear in more
   than one group).  Each group is labeled with a set of characters that
   every position in the group matches (taking into account, if necessary,
   preceding context information of s).  For each group, find the union
   of the its elements' follows.  This set is the set of positions of the
   new state.  For each character in the group's label, set the transition
   on this character to be to a state corresponding to the set's positions,
   and its associated backward context information, if necessary.

   If we are building a searching matcher, we include the positions of state
   0 in every state.

   The collection of groups is constructed by building an equivalence-class
   partition of the positions of s.

   For each position, find the set of characters C that it matches.  Eliminate
   any characters from C that fail on grounds of backward context.

   Search through the groups, looking for a group whose label L has nonempty
   intersection with C.  If L - C is nonempty, create a new group labeled
   L - C and having the same positions as the current group, and set L to
   the intersection of L and C.  Insert the position in this group, set
   C = C - L, and resume scanning.

   If after comparing with every group there are characters remaining in C,
   create a new group labeled with the characters of C and insert this
   position in that group.  */
void
dfastate (state_num s, struct dfa *d, state_num trans[])
{
  leaf_set grps[NOTCHAR];       /* As many as will ever be needed.  */
  charclass labels[NOTCHAR];    /* Labels corresponding to the groups.  */
  size_t ngrps = 0;             /* Number of groups actually used.  */
  position pos;                 /* Current position being considered.  */
  charclass matches;            /* Set of matching characters.  */
  charclass_word matchesf;	/* Nonzero if matches is nonempty.  */
  charclass intersect;          /* Intersection with some label set.  */
  charclass_word intersectf;	/* Nonzero if intersect is nonempty.  */
  charclass leftovers;          /* Stuff in the label that didn't match.  */
  charclass_word leftoversf;	/* Nonzero if leftovers is nonempty.  */
  position_set follows;         /* Union of the follows of some group.  */
  position_set tmp;             /* Temporary space for merging sets.  */
  int possible_contexts;        /* Contexts that this group can match.  */
  int separate_contexts;        /* Context that new state wants to know.  */
  state_num state;              /* New state.  */
  state_num state_newline;      /* New state on a newline transition.  */
  state_num state_letter;       /* New state on a letter transition.  */
  bool next_isnt_1st_byte = false; /* We can't add state0.  */
  size_t i, j, k;

  zeroset (matches);

  for (i = 0; i < d->states[s].elems.nelem; ++i)
    {
      pos = d->states[s].elems.elems[i];
      if (d->tokens[pos.index] >= 0 && d->tokens[pos.index] < NOTCHAR)
        setbit (d->tokens[pos.index], matches);
      else if (d->tokens[pos.index] >= CSET)
        copyset (d->charclasses[d->tokens[pos.index] - CSET], matches);
      else
        {
          if (d->tokens[pos.index] == MBCSET
              || d->tokens[pos.index] == ANYCHAR)
            {
              /* MB_CUR_MAX > 1 */
              if (d->tokens[pos.index] == MBCSET)
                d->states[s].has_mbcset = true;
              /* ANYCHAR and MBCSET must match with a single character, so we
                 must put it to d->states[s].mbps, which contains the positions
                 which can match with a single character not a byte.  */
              if (d->states[s].mbps.nelem == 0)
                alloc_position_set (&d->states[s].mbps, 1);
              insert (pos, &(d->states[s].mbps));
            }
          continue;
        }

      /* Some characters may need to be eliminated from matches because
         they fail in the current context.  */
      if (pos.constraint != NO_CONSTRAINT)
        {
          if (!SUCCEEDS_IN_CONTEXT (pos.constraint,
                                    d->states[s].context, CTX_NEWLINE))
            for (j = 0; j < CHARCLASS_WORDS; ++j)
              matches[j] &= ~newline[j];
          if (!SUCCEEDS_IN_CONTEXT (pos.constraint,
                                    d->states[s].context, CTX_LETTER))
            for (j = 0; j < CHARCLASS_WORDS; ++j)
              matches[j] &= ~letters[j];
          if (!SUCCEEDS_IN_CONTEXT (pos.constraint,
                                    d->states[s].context, CTX_NONE))
            for (j = 0; j < CHARCLASS_WORDS; ++j)
              matches[j] &= letters[j] | newline[j];

          /* If there are no characters left, there's no point in going on.  */
          for (j = 0; j < CHARCLASS_WORDS && !matches[j]; ++j)
            continue;
          if (j == CHARCLASS_WORDS)
            continue;
        }

      for (j = 0; j < ngrps; ++j)
        {
          /* If matches contains a single character only, and the current
             group's label doesn't contain that character, go on to the
             next group.  */
          if (d->tokens[pos.index] >= 0 && d->tokens[pos.index] < NOTCHAR
              && !tstbit (d->tokens[pos.index], labels[j]))
            continue;

          /* Check if this group's label has a nonempty intersection with
             matches.  */
          intersectf = 0;
          for (k = 0; k < CHARCLASS_WORDS; ++k)
            intersectf |= intersect[k] = matches[k] & labels[j][k];
          if (!intersectf)
            continue;

          /* It does; now find the set differences both ways.  */
          leftoversf = matchesf = 0;
          for (k = 0; k < CHARCLASS_WORDS; ++k)
            {
              /* Even an optimizing compiler can't know this for sure.  */
              charclass_word match = matches[k], label = labels[j][k];

              leftoversf |= leftovers[k] = ~match & label;
              matchesf |= matches[k] = match & ~label;
            }

          /* If there were leftovers, create a new group labeled with them.  */
          if (leftoversf)
            {
              copyset (leftovers, labels[ngrps]);
              copyset (intersect, labels[j]);
              grps[ngrps].elems = xnmalloc (d->nleaves,
                                            sizeof *grps[ngrps].elems);
              memcpy (grps[ngrps].elems, grps[j].elems,
                      sizeof (grps[j].elems[0]) * grps[j].nelem);
              grps[ngrps].nelem = grps[j].nelem;
              ++ngrps;
            }

          /* Put the position in the current group.  The constraint is
             irrelevant here.  */
          grps[j].elems[grps[j].nelem++] = pos.index;

          /* If every character matching the current position has been
             accounted for, we're done.  */
          if (!matchesf)
            break;
        }

      /* If we've passed the last group, and there are still characters
         unaccounted for, then we'll have to create a new group.  */
      if (j == ngrps)
        {
          copyset (matches, labels[ngrps]);
          zeroset (matches);
          grps[ngrps].elems = xnmalloc (d->nleaves, sizeof *grps[ngrps].elems);
          grps[ngrps].nelem = 1;
          grps[ngrps].elems[0] = pos.index;
          ++ngrps;
        }
    }

  alloc_position_set (&follows, d->nleaves);
  alloc_position_set (&tmp, d->nleaves);

  /* If we are a searching matcher, the default transition is to a state
     containing the positions of state 0, otherwise the default transition
     is to fail miserably.  */
  if (d->searchflag)
    {
      /* Find the state(s) corresponding to the positions of state 0.  */
      copy (&d->states[0].elems, &follows);
      separate_contexts = state_separate_contexts (&follows);
      state = state_index (d, &follows, separate_contexts ^ CTX_ANY);
      if (separate_contexts & CTX_NEWLINE)
        state_newline = state_index (d, &follows, CTX_NEWLINE);
      else
        state_newline = state;
      if (separate_contexts & CTX_LETTER)
        state_letter = state_index (d, &follows, CTX_LETTER);
      else
        state_letter = state;

      for (i = 0; i < NOTCHAR; ++i)
        trans[i] = (IS_WORD_CONSTITUENT (i)) ? state_letter : state;
      trans[eolbyte] = state_newline;
    }
  else
    for (i = 0; i < NOTCHAR; ++i)
      trans[i] = -1;

  for (i = 0; i < ngrps; ++i)
    {
      follows.nelem = 0;

      /* Find the union of the follows of the positions of the group.
         This is a hideously inefficient loop.  Fix it someday.  */
      for (j = 0; j < grps[i].nelem; ++j)
        for (k = 0; k < d->follows[grps[i].elems[j]].nelem; ++k)
          insert (d->follows[grps[i].elems[j]].elems[k], &follows);

      if (d->multibyte)
        {
          /* If a token in follows.elems is not 1st byte of a multibyte
             character, or the states of follows must accept the bytes
             which are not 1st byte of the multibyte character.
             Then, if a state of follows encounter a byte, it must not be
             a 1st byte of a multibyte character nor single byte character.
             We cansel to add state[0].follows to next state, because
             state[0] must accept 1st-byte

             For example, we assume <sb a> is a certain single byte
             character, <mb A> is a certain multibyte character, and the
             codepoint of <sb a> equals the 2nd byte of the codepoint of
             <mb A>.
             When state[0] accepts <sb a>, state[i] transit to state[i+1]
             by accepting accepts 1st byte of <mb A>, and state[i+1]
             accepts 2nd byte of <mb A>, if state[i+1] encounter the
             codepoint of <sb a>, it must not be <sb a> but 2nd byte of
             <mb A>, so we cannot add state[0].  */

          next_isnt_1st_byte = false;
          for (j = 0; j < follows.nelem; ++j)
            {
              if (!(d->multibyte_prop[follows.elems[j].index] & 1))
                {
                  next_isnt_1st_byte = true;
                  break;
                }
            }
        }

      /* If we are building a searching matcher, throw in the positions
         of state 0 as well.  */
      if (d->searchflag && (!d->multibyte || !next_isnt_1st_byte))
        {
          merge (&d->states[0].elems, &follows, &tmp);
          copy (&tmp, &follows);
        }

      /* Find out if the new state will want any context information.  */
      possible_contexts = charclass_context (labels[i]);
      separate_contexts = state_separate_contexts (&follows);

      /* Find the state(s) corresponding to the union of the follows.  */
      if ((separate_contexts & possible_contexts) != possible_contexts)
        state = state_index (d, &follows, separate_contexts ^ CTX_ANY);
      else
        state = -1;
      if (separate_contexts & possible_contexts & CTX_NEWLINE)
        state_newline = state_index (d, &follows, CTX_NEWLINE);
      else
        state_newline = state;
      if (separate_contexts & possible_contexts & CTX_LETTER)
        state_letter = state_index (d, &follows, CTX_LETTER);
      else
        state_letter = state;

      /* Set the transitions for each character in the current label.  */
      for (j = 0; j < CHARCLASS_WORDS; ++j)
        for (k = 0; k < CHARCLASS_WORD_BITS; ++k)
          if (labels[i][j] >> k & 1)
            {
              int c = j * CHARCLASS_WORD_BITS + k;

              if (c == eolbyte)
                trans[c] = state_newline;
              else if (IS_WORD_CONSTITUENT (c))
                trans[c] = state_letter;
              else if (c < NOTCHAR)
                trans[c] = state;
            }
    }

  for (i = 0; i < ngrps; ++i)
    free (grps[i].elems);
  free (follows.elems);
  free (tmp.elems);
}

/* Make sure D's state arrays are large enough to hold NEW_STATE.  */
static void
realloc_trans_if_necessary (struct dfa *d, state_num new_state)
{
  state_num oldalloc = d->tralloc;
  if (oldalloc <= new_state)
    {
      state_num **realtrans = d->trans ? d->trans - 1 : NULL;
      size_t newalloc, newalloc1;
      newalloc1 = new_state + 1;
      realtrans = x2nrealloc (realtrans, &newalloc1, sizeof *realtrans);
      realtrans[0] = NULL;
      d->trans = realtrans + 1;
      d->tralloc = newalloc = newalloc1 - 1;
      d->fails = xnrealloc (d->fails, newalloc, sizeof *d->fails);
      d->success = xnrealloc (d->success, newalloc, sizeof *d->success);
      d->newlines = xnrealloc (d->newlines, newalloc, sizeof *d->newlines);
      for (; oldalloc < newalloc; oldalloc++)
        {
          d->trans[oldalloc] = NULL;
          d->fails[oldalloc] = NULL;
        }
    }
}

/* Some routines for manipulating a compiled dfa's transition tables.
   Each state may or may not have a transition table; if it does, and it
   is a non-accepting state, then d->trans[state] points to its table.
   If it is an accepting state then d->fails[state] points to its table.
   If it has no table at all, then d->trans[state] is NULL.
   TODO: Improve this comment, get rid of the unnecessary redundancy.  */

static void
build_state (state_num s, struct dfa *d)
{
  state_num *trans;             /* The new transition table.  */
  state_num i, maxstate;

  /* Set an upper limit on the number of transition tables that will ever
     exist at once.  1024 is arbitrary.  The idea is that the frequently
     used transition tables will be quickly rebuilt, whereas the ones that
     were only needed once or twice will be cleared away.  However, do not
     clear the initial D->min_trcount states, since they are always used.  */
  if (d->trcount >= 1024)
    {
      for (i = d->min_trcount; i < d->tralloc; ++i)
        {
          free (d->trans[i]);
          free (d->fails[i]);
          d->trans[i] = d->fails[i] = NULL;
        }
      d->trcount = d->min_trcount;
    }

  ++d->trcount;

  /* Set up the success bits for this state.  */
  d->success[s] = 0;
  if (ACCEPTS_IN_CONTEXT (d->states[s].context, CTX_NEWLINE, s, *d))
    d->success[s] |= CTX_NEWLINE;
  if (ACCEPTS_IN_CONTEXT (d->states[s].context, CTX_LETTER, s, *d))
    d->success[s] |= CTX_LETTER;
  if (ACCEPTS_IN_CONTEXT (d->states[s].context, CTX_NONE, s, *d))
    d->success[s] |= CTX_NONE;

  trans = xmalloc (NOTCHAR * sizeof *trans);
  dfastate (s, d, trans);

  /* Now go through the new transition table, and make sure that the trans
     and fail arrays are allocated large enough to hold a pointer for the
     largest state mentioned in the table.  */
  maxstate = -1;
  for (i = 0; i < NOTCHAR; ++i)
    if (maxstate < trans[i])
      maxstate = trans[i];
  realloc_trans_if_necessary (d, maxstate);

  /* Keep the newline transition in a special place so we can use it as
     a sentinel.  */
  d->newlines[s] = trans[eolbyte];
  trans[eolbyte] = -1;

  if (ACCEPTING (s, *d))
    d->fails[s] = trans;
  else
    d->trans[s] = trans;
}

/* Multibyte character handling sub-routines for dfaexec.  */

/* Return values of transit_state_singlebyte, and
   transit_state_consume_1char.  */
typedef enum
{
  TRANSIT_STATE_IN_PROGRESS,    /* State transition has not finished.  */
  TRANSIT_STATE_DONE,           /* State transition has finished.  */
  TRANSIT_STATE_END_BUFFER      /* Reach the end of the buffer.  */
} status_transit_state;

/* Consume a single byte and transit state from 's' to '*next_state'.
   This function is almost same as the state transition routin in dfaexec.
   But state transition is done just once, otherwise matching succeed or
   reach the end of the buffer.  */
static status_transit_state
transit_state_singlebyte (struct dfa *d, state_num s, unsigned char const *p,
                          state_num * next_state)
{
  state_num *t;
  state_num works = s;

  status_transit_state rval = TRANSIT_STATE_IN_PROGRESS;

  while (rval == TRANSIT_STATE_IN_PROGRESS)
    {
      if ((t = d->trans[works]) != NULL)
        {
          works = t[*p];
          rval = TRANSIT_STATE_DONE;
          if (works < 0)
            works = 0;
        }
      else if (works < 0)
        works = 0;
      else if (d->fails[works])
        {
          works = d->fails[works][*p];
          rval = TRANSIT_STATE_DONE;
        }
      else
        {
          build_state (works, d);
        }
    }
  *next_state = works;
  return rval;
}

/* Match a "." against the current context.  Return the length of the
   match, in bytes.  POS is the position of the ".".  */
static int
match_anychar (struct dfa *d, state_num s, position pos,
               wint_t wc, size_t mbclen)
{
  int context;

  /* Check syntax bits.  */
  if (wc == (wchar_t) eolbyte)
    {
      if (!(syntax_bits & RE_DOT_NEWLINE))
        return 0;
    }
  else if (wc == (wchar_t) '\0')
    {
      if (syntax_bits & RE_DOT_NOT_NULL)
        return 0;
    }
  else if (wc == WEOF)
    return 0;

  context = wchar_context (wc);
  if (!SUCCEEDS_IN_CONTEXT (pos.constraint, d->states[s].context, context))
    return 0;

  return mbclen;
}

/* Match a bracket expression against the current context.
   Return the length of the match, in bytes.
   POS is the position of the bracket expression.  */
static int
match_mb_charset (struct dfa *d, state_num s, position pos,
                  char const *p, wint_t wc, size_t match_len)
{
  size_t i;
  bool match;              /* Matching succeeded.  */
  int op_len;              /* Length of the operator.  */
  char buffer[128];

  /* Pointer to the structure to which we are currently referring.  */
  struct mb_char_classes *work_mbc;

  int context;

  /* Check syntax bits.  */
  if (wc == WEOF)
    return 0;

  context = wchar_context (wc);
  if (!SUCCEEDS_IN_CONTEXT (pos.constraint, d->states[s].context, context))
    return 0;

  /* Assign the current referring operator to work_mbc.  */
  work_mbc = &(d->mbcsets[(d->multibyte_prop[pos.index]) >> 2]);
  match = !work_mbc->invert;

  /* Match in range 0-255?  */
  if (wc < NOTCHAR && work_mbc->cset != -1
      && tstbit (to_uchar (wc), d->charclasses[work_mbc->cset]))
    goto charset_matched;

  /* match with a character class?  */
  for (i = 0; i < work_mbc->nch_classes; i++)
    {
      if (iswctype ((wint_t) wc, work_mbc->ch_classes[i]))
        goto charset_matched;
    }

  strncpy (buffer, p, match_len);
  buffer[match_len] = '\0';

  /* match with an equivalence class?  */
  for (i = 0; i < work_mbc->nequivs; i++)
    {
      op_len = strlen (work_mbc->equivs[i]);
      strncpy (buffer, p, op_len);
      buffer[op_len] = '\0';
      if (strcoll (work_mbc->equivs[i], buffer) == 0)
        {
          match_len = op_len;
          goto charset_matched;
        }
    }

  /* match with a collating element?  */
  for (i = 0; i < work_mbc->ncoll_elems; i++)
    {
      op_len = strlen (work_mbc->coll_elems[i]);
      strncpy (buffer, p, op_len);
      buffer[op_len] = '\0';

      if (strcoll (work_mbc->coll_elems[i], buffer) == 0)
        {
          match_len = op_len;
          goto charset_matched;
        }
    }

  /* match with a range?  */
  for (i = 0; i < work_mbc->nranges; i++)
    {
      if (work_mbc->ranges[i].beg <= wc && wc <= work_mbc->ranges[i].end)
        goto charset_matched;
    }

  /* match with a character?  */
  for (i = 0; i < work_mbc->nchars; i++)
    {
      if (wc == work_mbc->chars[i])
        goto charset_matched;
    }

  match = !match;

charset_matched:
  return match ? match_len : 0;
}

/* Check whether each of 'd->states[s].mbps.elem' can match.  Then return the
   array which corresponds to 'd->states[s].mbps.elem'; each element of the
   array contains the number of bytes with which the element can match.

   The caller MUST free the array which this function return.  */
static int *
check_matching_with_multibyte_ops (struct dfa *d, state_num s,
                                   char const *p, wint_t wc, size_t mbclen)
{
  size_t i;
  int *rarray;

  rarray = d->mb_match_lens;
  for (i = 0; i < d->states[s].mbps.nelem; ++i)
    {
      position pos = d->states[s].mbps.elems[i];
      switch (d->tokens[pos.index])
        {
        case ANYCHAR:
          rarray[i] = match_anychar (d, s, pos, wc, mbclen);
          break;
        case MBCSET:
          rarray[i] = match_mb_charset (d, s, pos, p, wc, mbclen);
          break;
        default:
          break;                /* cannot happen.  */
        }
    }
  return rarray;
}

/* Consume a single character and enumerate all of the positions which can
   be the next position from the state 's'.

   'match_lens' is the input.  It can be NULL, but it can also be the output
   of check_matching_with_multibyte_ops for optimization.

   'mbclen' and 'pps' are the output.  'mbclen' is the length of the
   character consumed, and 'pps' is the set this function enumerates.  */
static status_transit_state
transit_state_consume_1char (struct dfa *d, state_num s,
                             unsigned char const **pp,
                             wint_t wc, size_t mbclen,
                             int *match_lens)
{
  size_t i, j;
  int k;
  state_num s1, s2;
  status_transit_state rs = TRANSIT_STATE_DONE;

  if (! match_lens && d->states[s].mbps.nelem != 0)
    match_lens = check_matching_with_multibyte_ops (d, s, (char const *) *pp,
                                                    wc, mbclen);

  /* Calculate the state which can be reached from the state 's' by
     consuming 'mbclen' single bytes from the buffer.  */
  s1 = s;
  for (k = 0; k < mbclen; k++)
    {
      s2 = s1;
      rs = transit_state_singlebyte (d, s2, (*pp)++, &s1);
    }
  copy (&d->states[s1].elems, &d->mb_follows);

  /* Add all of the positions which can be reached from 's' by consuming
     a single character.  */
  for (i = 0; i < d->states[s].mbps.nelem; i++)
    {
      if (match_lens[i] == mbclen)
        for (j = 0; j < d->follows[d->states[s].mbps.elems[i].index].nelem;
             j++)
          insert (d->follows[d->states[s].mbps.elems[i].index].elems[j],
                  &d->mb_follows);
    }

  /* FIXME: this return value is always ignored.  */
  return rs;
}

/* Transit state from s, then return new state and update the pointer of the
   buffer.  This function is for some operator which can match with a multi-
   byte character or a collating element (which may be multi characters).  */
static state_num
transit_state (struct dfa *d, state_num s, unsigned char const **pp,
               unsigned char const *end)
{
  state_num s1;
  int mbclen;  /* The length of current input multibyte character.  */
  int maxlen = 0;
  size_t i, j;
  int *match_lens = NULL;
  size_t nelem = d->states[s].mbps.nelem;       /* Just a alias.  */
  unsigned char const *p1 = *pp;
  wint_t wc;

  if (nelem > 0)
    /* This state has (a) multibyte operator(s).
       We check whether each of them can match or not.  */
    {
      /* Note: caller must free the return value of this function.  */
      mbclen = mbs_to_wchar (&wc, (char const *) *pp, end - *pp, d);
      match_lens = check_matching_with_multibyte_ops (d, s, (char const *) *pp,
                                                      wc, mbclen);

      for (i = 0; i < nelem; i++)
        /* Search the operator which match the longest string,
           in this state.  */
        {
          if (match_lens[i] > maxlen)
            maxlen = match_lens[i];
        }
    }

  if (nelem == 0 || maxlen == 0)
    /* This state has no multibyte operator which can match.
       We need to check only one single byte character.  */
    {
      status_transit_state rs;
      rs = transit_state_singlebyte (d, s, *pp, &s1);

      /* We must update the pointer if state transition succeeded.  */
      if (rs == TRANSIT_STATE_DONE)
        ++*pp;

      return s1;
    }

  /* This state has some operators which can match a multibyte character.  */
  d->mb_follows.nelem = 0;

  /* 'maxlen' may be longer than the length of a character, because it may
     not be a character but a (multi character) collating element.
     We enumerate all of the positions which 's' can reach by consuming
     'maxlen' bytes.  */
  transit_state_consume_1char (d, s, pp, wc, mbclen, match_lens);

  s1 = state_index (d, &d->mb_follows, wchar_context (wc));
  realloc_trans_if_necessary (d, s1);

  while (*pp - p1 < maxlen)
    {
      mbclen = mbs_to_wchar (&wc, (char const *) *pp, end - *pp, d);
      transit_state_consume_1char (d, s1, pp, wc, mbclen, NULL);

      for (i = 0; i < nelem; i++)
        {
          if (match_lens[i] == *pp - p1)
            for (j = 0;
                 j < d->follows[d->states[s1].mbps.elems[i].index].nelem; j++)
              insert (d->follows[d->states[s1].mbps.elems[i].index].elems[j],
                      &d->mb_follows);
        }

      s1 = state_index (d, &d->mb_follows, wchar_context (wc));
      realloc_trans_if_necessary (d, s1);
    }
  return s1;
}

/* The initial state may encounter a byte which is not a single byte character
   nor the first byte of a multibyte character.  But it is incorrect for the
   initial state to accept such a byte.  For example, in Shift JIS the regular
   expression "\\" accepts the codepoint 0x5c, but should not accept the second
   byte of the codepoint 0x815c.  Then the initial state must skip the bytes
   that are not a single byte character nor the first byte of a multibyte
   character.

   Given DFA state d, use mbs_to_wchar to advance MBP until it reaches or
   exceeds P.  If WCP is non-NULL, set *WCP to the final wide character
   processed, or if no wide character is processed, set it to WEOF.
   Both P and MBP must be no larger than END.  */
static unsigned char const *
skip_remains_mb (struct dfa *d, unsigned char const *p,
                 unsigned char const *mbp, char const *end, wint_t *wcp)
{
  wint_t wc = WEOF;
  while (mbp < p)
    mbp += mbs_to_wchar (&wc, (char const *) mbp,
                         end - (char const *) mbp, d);
  if (wcp != NULL)
    *wcp = wc;
  return mbp;
}

/* Search through a buffer looking for a match to the given struct dfa.
   Find the first occurrence of a string matching the regexp in the
   buffer, and the shortest possible version thereof.  Return a pointer to
   the first character after the match, or NULL if none is found.  BEGIN
   points to the beginning of the buffer, and END points to the first byte
   after its end.  Note however that we store a sentinel byte (usually
   newline) in *END, so the actual buffer must be one byte longer.
   When ALLOW_NL is nonzero, newlines may appear in the matching string.
   If COUNT is non-NULL, increment *COUNT once for each newline processed.
   Finally, if BACKREF is non-NULL set *BACKREF to indicate whether we
   encountered a back-reference (1) or not (0).  The caller may use this
   to decide whether to fall back on a backtracking matcher.

   If MULTIBYTE, the input consists of multibyte characters and/or
   encoding-error bytes.  Otherwise, the input consists of single-byte
   characters.  */
static inline char *
dfaexec_main (struct dfa *d, char const *begin, char *end,
             int allow_nl, size_t *count, int *backref, bool multibyte)
{
  state_num s, s1;              /* Current state.  */
  unsigned char const *p, *mbp; /* Current input character.  */
  state_num **trans, *t;        /* Copy of d->trans so it can be optimized
                                   into a register.  */
  unsigned char eol = eolbyte;  /* Likewise for eolbyte.  */
  unsigned char saved_end;
  size_t nlcount = 0;

  if (!d->tralloc)
    {
      realloc_trans_if_necessary (d, 1);
      build_state (0, d);
    }

  s = s1 = 0;
  p = mbp = (unsigned char const *) begin;
  trans = d->trans;
  saved_end = *(unsigned char *) end;
  *end = eol;

  if (multibyte)
    {
      memset (&d->mbs, 0, sizeof d->mbs);
      if (! d->mb_match_lens)
        {
          d->mb_match_lens = xnmalloc (d->nleaves, sizeof *d->mb_match_lens);
          alloc_position_set (&d->mb_follows, d->nleaves);
        }
    }

  for (;;)
    {
      if (multibyte)
        {
          while ((t = trans[s]) != NULL)
            {
              s1 = s;

              if (s < d->min_trcount)
                {
                  if (d->min_trcount == 1)
                    {
                      if (d->states[s].mbps.nelem == 0)
                        {
                          do
                            {
                              while (t[*p] == 0)
                                p++;
                              p = mbp = skip_remains_mb (d, p, mbp, end, NULL);
                            }
                          while (t[*p] == 0);
                        }
                      else
                        p = mbp = skip_remains_mb (d, p, mbp, end, NULL);
                    }
                  else
                    {
                      wint_t wc;
                      mbp = skip_remains_mb (d, p, mbp, end, &wc);

                      /* If d->min_trcount is greater than 1, maybe
                         transit to another initial state after skip.  */
                      if (p < mbp)
                        {
                          int context = wchar_context (wc);
                          if (context == CTX_LETTER)
                            s = d->initstate_letter;
                          else
                            /* It's CTX_NONE.  CTX_NEWLINE cannot happen,
                               as we assume that a newline is always a
                               single byte character.  */
                            s = d->initstate_others;
                          p = mbp;
                          s1 = s;
                        }
                    }
                }

              if (d->states[s].mbps.nelem == 0)
                {
                  s = t[*p++];
                  continue;
                }

              /* The following code is used twice.
                 Use a macro to avoid the risk that they diverge.  */
#define State_transition()                                              \
  do {                                                                  \
              /* Falling back to the glibc matcher in this case gives   \
                 better performance (up to 25% better on [a-z], for     \
                 example) and enables support for collating symbols and \
                 equivalence classes.  */                               \
              if (d->states[s].has_mbcset && backref)                   \
                {                                                       \
                  *backref = 1;                                         \
                  goto done;                                            \
                }                                                       \
                                                                        \
              /* Can match with a multibyte character (and multi-character \
                 collating element).  Transition table might be updated.  */ \
              s = transit_state (d, s, &p, (unsigned char *) end);      \
                                                                        \
              /* If previous character is newline after a transition    \
                 for ANYCHAR or MBCSET in non-UTF8 multibyte locales,   \
                 check whether current position is beyond the end of    \
                 the input buffer.  Also, transit to initial state if   \
                 !ALLOW_NL, even if RE_DOT_NEWLINE is set. */           \
              if (p[-1] == eol)                                         \
                {                                                       \
                  if ((char *) p > end)                                 \
                    {                                                   \
                      p = NULL;                                         \
                      goto done;                                        \
                    }                                                   \
                                                                        \
                  nlcount++;                                            \
                                                                        \
                  if (!allow_nl)                                        \
                    s = 0;                                              \
                }                                                       \
                                                                        \
              mbp = p;                                                  \
              trans = d->trans;                                         \
  } while (0)

              State_transition();
            }
        }
      else
        {
          if (s == 0 && (t = trans[s]) != NULL)
            {
              while (t[*p] == 0)
                p++;
              s1 = 0;
              s = t[*p++];
            }

          while ((t = trans[s]) != NULL)
            {
              s1 = t[*p++];
              if ((t = trans[s1]) == NULL)
                {
                  state_num tmp = s;
                  s = s1;
                  s1 = tmp;     /* swap */
                  break;
                }
              s = t[*p++];
            }
        }

      if (s < 0)
        {
          if ((char *) p > end || p[-1] != eol || d->newlines[s1] < 0)
            {
              p = NULL;
              goto done;
            }

          /* The previous character was a newline, count it, and skip
             checking of multibyte character boundary until here.  */
          nlcount++;
          mbp = p;

          s = allow_nl ? d->newlines[s1] : 0;
        }

      if (d->fails[s])
        {
          if (d->success[s] & sbit[*p])
            {
              if (backref)
                *backref = d->states[s].has_backref;
              goto done;
            }

          s1 = s;
          if (multibyte)
            State_transition();
          else
            s = d->fails[s][*p++];
        }
      else
        {
          if (!d->trans[s])
            build_state (s, d);
          trans = d->trans;
        }
    }

 done:
  if (count)
    *count += nlcount;
  *end = saved_end;
  return (char *) p;
}

/* Specialized versions of dfaexec_main for multibyte and single-byte
   cases.  This is for performance.  */

static char *
dfaexec_mb (struct dfa *d, char const *begin, char *end,
            int allow_nl, size_t *count, int *backref)
{
  return dfaexec_main (d, begin, end, allow_nl, count, backref, true);
}

static char *
dfaexec_sb (struct dfa *d, char const *begin, char *end,
            int allow_nl, size_t *count, int *backref)
{
  return dfaexec_main (d, begin, end, allow_nl, count, backref, false);
}

/* Like dfaexec_main (D, BEGIN, END, ALLOW_NL, COUNT, BACKREF, D->multibyte),
   but faster.  */

char *
dfaexec (struct dfa *d, char const *begin, char *end,
         int allow_nl, size_t *count, int *backref)
{
  return d->dfaexec (d, begin, end, allow_nl, count, backref);
}

struct dfa *
dfasuperset (struct dfa const *d)
{
  return d->superset;
}

bool
dfaisfast (struct dfa const *d)
{
  return d->fast;
}

static void
free_mbdata (struct dfa *d)
{
  size_t i;

  free (d->multibyte_prop);

  for (i = 0; i < d->nmbcsets; ++i)
    {
      size_t j;
      struct mb_char_classes *p = &(d->mbcsets[i]);
      free (p->chars);
      free (p->ch_classes);
      free (p->ranges);

      for (j = 0; j < p->nequivs; ++j)
        free (p->equivs[j]);
      free (p->equivs);

      for (j = 0; j < p->ncoll_elems; ++j)
        free (p->coll_elems[j]);
      free (p->coll_elems);
    }

  free (d->mbcsets);
  free (d->mb_follows.elems);
  free (d->mb_match_lens);
  d->mb_match_lens = NULL;
}

/* Initialize the components of a dfa that the other routines don't
   initialize for themselves.  */
void
dfainit (struct dfa *d)
{
  memset (d, 0, sizeof *d);
  d->multibyte = MB_CUR_MAX > 1;
  d->dfaexec = d->multibyte ? dfaexec_mb : dfaexec_sb;
  d->fast = !d->multibyte;
}

static void
dfaoptimize (struct dfa *d)
{
  size_t i;
  bool have_backref = false;

  if (!using_utf8 ())
    return;

  for (i = 0; i < d->tindex; ++i)
    {
      switch (d->tokens[i])
        {
        case ANYCHAR:
          /* Lowered.  */
          abort ();
        case BACKREF:
          have_backref = true;
          break;
        case MBCSET:
          /* Requires multi-byte algorithm.  */
          return;
        default:
          break;
        }
    }

  if (!have_backref && d->superset)
    {
      /* The superset DFA is not likely to be much faster, so remove it.  */
      dfafree (d->superset);
      free (d->superset);
      d->superset = NULL;
    }

  free_mbdata (d);
  d->multibyte = false;
  d->dfaexec = dfaexec_sb;
}

static void
dfassbuild (struct dfa *d)
{
  size_t i, j;
  charclass ccl;
  bool have_achar = false;
  bool have_nchar = false;
  struct dfa *sup = dfaalloc ();

  *sup = *d;
  sup->multibyte = false;
  sup->dfaexec = dfaexec_sb;
  sup->multibyte_prop = NULL;
  sup->mbcsets = NULL;
  sup->superset = NULL;
  sup->states = NULL;
  sup->sindex = 0;
  sup->follows = NULL;
  sup->tralloc = 0;
  sup->trans = NULL;
  sup->fails = NULL;
  sup->success = NULL;
  sup->newlines = NULL;
  sup->musts = NULL;

  sup->charclasses = xnmalloc (sup->calloc, sizeof *sup->charclasses);
  if (d->cindex)
    {
      memcpy (sup->charclasses, d->charclasses,
              d->cindex * sizeof *sup->charclasses);
    }

  sup->tokens = xnmalloc (d->tindex, 2 * sizeof *sup->tokens);
  sup->talloc = d->tindex * 2;

  for (i = j = 0; i < d->tindex; i++)
    {
      switch (d->tokens[i])
        {
        case ANYCHAR:
        case MBCSET:
        case BACKREF:
          zeroset (ccl);
          notset (ccl);
          sup->tokens[j++] = CSET + dfa_charclass_index (sup, ccl);
          sup->tokens[j++] = STAR;
          if (d->tokens[i + 1] == QMARK || d->tokens[i + 1] == STAR
              || d->tokens[i + 1] == PLUS)
            i++;
          have_achar = true;
          break;
        case BEGWORD:
        case ENDWORD:
        case LIMWORD:
        case NOTLIMWORD:
          if (d->multibyte)
            {
              /* These constraints aren't supported in a multibyte locale.
                 Ignore them in the superset DFA, and treat them as
                 backreferences in the main DFA.  */
              sup->tokens[j++] = EMPTY;
              d->tokens[i] = BACKREF;
              break;
            }
        default:
          sup->tokens[j++] = d->tokens[i];
          if ((0 <= d->tokens[i] && d->tokens[i] < NOTCHAR)
              || d->tokens[i] >= CSET)
            have_nchar = true;
          break;
        }
    }
  sup->tindex = j;

  if (have_nchar && (have_achar || d->multibyte))
    d->superset = sup;
  else
    {
      dfafree (sup);
      free (sup);
    }
}

/* Parse and analyze a single string of the given length.  */
void
dfacomp (char const *s, size_t len, struct dfa *d, int searchflag)
{
  dfainit (d);
  dfambcache (d);
  dfaparse (s, len, d);
  dfamust (d);
  dfassbuild (d);
  dfaoptimize (d);
  dfaanalyze (d, searchflag);
  if (d->superset)
    {
      d->fast = true;
      dfaanalyze (d->superset, searchflag);
    }
}

/* Free the storage held by the components of a dfa.  */
void
dfafree (struct dfa *d)
{
  size_t i;
  struct dfamust *dm, *ndm;

  free (d->charclasses);
  free (d->tokens);

  if (d->multibyte)
    free_mbdata (d);

  for (i = 0; i < d->sindex; ++i)
    {
      free (d->states[i].elems.elems);
      free (d->states[i].mbps.elems);
    }
  free (d->states);

  if (d->follows)
    {
      for (i = 0; i < d->tindex; ++i)
        free (d->follows[i].elems);
      free (d->follows);
    }

  if (d->trans)
    {
      for (i = 0; i < d->tralloc; ++i)
        {
          free (d->trans[i]);
          free (d->fails[i]);
        }

      free (d->trans - 1);
      free (d->fails);
      free (d->newlines);
      free (d->success);
    }

  for (dm = d->musts; dm; dm = ndm)
    {
      ndm = dm->next;
      free (dm->must);
      free (dm);
    }

  if (d->superset)
    dfafree (d->superset);
}

/* Having found the postfix representation of the regular expression,
   try to find a long sequence of characters that must appear in any line
   containing the r.e.
   Finding a "longest" sequence is beyond the scope here;
   we take an easy way out and hope for the best.
   (Take "(ab|a)b"--please.)

   We do a bottom-up calculation of sequences of characters that must appear
   in matches of r.e.'s represented by trees rooted at the nodes of the postfix
   representation:
        sequences that must appear at the left of the match ("left")
        sequences that must appear at the right of the match ("right")
        lists of sequences that must appear somewhere in the match ("in")
        sequences that must constitute the match ("is")

   When we get to the root of the tree, we use one of the longest of its
   calculated "in" sequences as our answer.  The sequence we find is returned in
   d->must (where "d" is the single argument passed to "dfamust");
   the length of the sequence is returned in d->mustn.

   The sequences calculated for the various types of node (in pseudo ANSI c)
   are shown below.  "p" is the operand of unary operators (and the left-hand
   operand of binary operators); "q" is the right-hand operand of binary
   operators.

   "ZERO" means "a zero-length sequence" below.

        Type	left		right		is		in
        ----	----		-----		--		--
        char c	# c		# c		# c		# c

        ANYCHAR	ZERO		ZERO		ZERO		ZERO

        MBCSET	ZERO		ZERO		ZERO		ZERO

        CSET	ZERO		ZERO		ZERO		ZERO

        STAR	ZERO		ZERO		ZERO		ZERO

        QMARK	ZERO		ZERO		ZERO		ZERO

        PLUS	p->left		p->right	ZERO		p->in

        CAT	(p->is==ZERO)?	(q->is==ZERO)?	(p->is!=ZERO &&	p->in plus
                p->left :	q->right :	q->is!=ZERO) ?	q->in plus
                p->is##q->left	p->right##q->is	p->is##q->is : p->right##q->left
                                                ZERO

        OR	longest common	longest common	(do p->is and substrings common
                leading		trailing	to q->is have same p->in and
                (sub)sequence	(sub)sequence	q->in length and content) ?
                of p->left	of p->right
                and q->left	and q->right	p->is : NULL

   If there's anything else we recognize in the tree, all four sequences get set
   to zero-length sequences.  If there's something we don't recognize in the
   tree, we just return a zero-length sequence.

   Break ties in favor of infrequent letters (choosing 'zzz' in preference to
   'aaa')?

   And ... is it here or someplace that we might ponder "optimizations" such as
        egrep 'psi|epsilon'	->	egrep 'psi'
        egrep 'pepsi|epsilon'	->	egrep 'epsi'
                                        (Yes, we now find "epsi" as a "string
                                        that must occur", but we might also
                                        simplify the *entire* r.e. being sought)
        grep '[c]'		->	grep 'c'
        grep '(ab|a)b'		->	grep 'ab'
        grep 'ab*'		->	grep 'a'
        grep 'a*b'		->	grep 'b'

   There are several issues:

   Is optimization easy (enough)?

   Does optimization actually accomplish anything,
   or is the automaton you get from "psi|epsilon" (for example)
   the same as the one you get from "psi" (for example)?

   Are optimizable r.e.'s likely to be used in real-life situations
   (something like 'ab*' is probably unlikely; something like is
   'psi|epsilon' is likelier)?  */

static char *
icatalloc (char *old, char const *new)
{
  char *result;
  size_t oldsize;
  size_t newsize = strlen (new);
  if (newsize == 0)
    return old;
  oldsize = strlen (old);
  result = xrealloc (old, oldsize + newsize + 1);
  memcpy (result + oldsize, new, newsize + 1);
  return result;
}

static void
freelist (char **cpp)
{
  while (*cpp)
    free (*cpp++);
}

static char **
enlist (char **cpp, char *new, size_t len)
{
  size_t i, j;
  new = memcpy (xmalloc (len + 1), new, len);
  new[len] = '\0';
  /* Is there already something in the list that's new (or longer)?  */
  for (i = 0; cpp[i] != NULL; ++i)
    if (strstr (cpp[i], new) != NULL)
      {
        free (new);
        return cpp;
      }
  /* Eliminate any obsoleted strings.  */
  j = 0;
  while (cpp[j] != NULL)
    if (strstr (new, cpp[j]) == NULL)
      ++j;
    else
      {
        free (cpp[j]);
        if (--i == j)
          break;
        cpp[j] = cpp[i];
        cpp[i] = NULL;
      }
  /* Add the new string.  */
  cpp = xnrealloc (cpp, i + 2, sizeof *cpp);
  cpp[i] = new;
  cpp[i + 1] = NULL;
  return cpp;
}

/* Given pointers to two strings, return a pointer to an allocated
   list of their distinct common substrings.  */
static char **
comsubs (char *left, char const *right)
{
  char **cpp = xzalloc (sizeof *cpp);
  char *lcp;

  for (lcp = left; *lcp != '\0'; ++lcp)
    {
      size_t len = 0;
      char *rcp = strchr (right, *lcp);
      while (rcp != NULL)
        {
          size_t i;
          for (i = 1; lcp[i] != '\0' && lcp[i] == rcp[i]; ++i)
            continue;
          if (i > len)
            len = i;
          rcp = strchr (rcp + 1, *lcp);
        }
      if (len != 0)
        cpp = enlist (cpp, lcp, len);
    }
  return cpp;
}

static char **
addlists (char **old, char **new)
{
  for (; *new; new++)
    old = enlist (old, *new, strlen (*new));
  return old;
}

/* Given two lists of substrings, return a new list giving substrings
   common to both.  */
static char **
inboth (char **left, char **right)
{
  char **both = xzalloc (sizeof *both);
  size_t lnum, rnum;

  for (lnum = 0; left[lnum] != NULL; ++lnum)
    {
      for (rnum = 0; right[rnum] != NULL; ++rnum)
        {
          char **temp = comsubs (left[lnum], right[rnum]);
          both = addlists (both, temp);
          freelist (temp);
          free (temp);
        }
    }
  return both;
}

typedef struct must must;

struct must
{
  char **in;
  char *left;
  char *right;
  char *is;
  bool begline;
  bool endline;
  must *prev;
};

static must *
allocmust (must *mp)
{
  must *new_mp = xmalloc (sizeof *new_mp);
  new_mp->in = xzalloc (sizeof *new_mp->in);
  new_mp->left = xzalloc (2);
  new_mp->right = xzalloc (2);
  new_mp->is = xzalloc (2);
  new_mp->begline = false;
  new_mp->endline = false;
  new_mp->prev = mp;
  return new_mp;
}

static void
resetmust (must *mp)
{
  freelist (mp->in);
  mp->in[0] = NULL;
  mp->left[0] = mp->right[0] = mp->is[0] = '\0';
  mp->begline = false;
  mp->endline = false;
}

static void
freemust (must *mp)
{
  freelist (mp->in);
  free (mp->in);
  free (mp->left);
  free (mp->right);
  free (mp->is);
  free (mp);
}

static void
dfamust (struct dfa *d)
{
  must *mp = NULL;
  char const *result = "";
  size_t ri;
  size_t i;
  bool exact = false;
  bool begline = false;
  bool endline = false;
  struct dfamust *dm;

  for (ri = 0; ri < d->tindex; ++ri)
    {
      token t = d->tokens[ri];
      switch (t)
        {
        case BEGLINE:
          mp = allocmust (mp);
          mp->begline = true;
          break;
        case ENDLINE:
          mp = allocmust (mp);
          mp->endline = true;
          break;
        case LPAREN:
        case RPAREN:
          assert (!"neither LPAREN nor RPAREN may appear here");

        case EMPTY:
        case BEGWORD:
        case ENDWORD:
        case LIMWORD:
        case NOTLIMWORD:
        case BACKREF:
        case ANYCHAR:
        case MBCSET:
          mp = allocmust (mp);
          break;

        case STAR:
        case QMARK:
          resetmust (mp);
          break;

        case OR:
          {
            char **new;
            must *rmp = mp;
            must *lmp = mp = mp->prev;
            size_t j, ln, rn, n;

            /* Guaranteed to be.  Unlikely, but ...  */
            if (STREQ (lmp->is, rmp->is))
              {
                lmp->begline &= rmp->begline;
                lmp->endline &= rmp->endline;
              }
            else
              {
                lmp->is[0] = '\0';
                lmp->begline = false;
                lmp->endline = false;
              }
            /* Left side--easy */
            i = 0;
            while (lmp->left[i] != '\0' && lmp->left[i] == rmp->left[i])
              ++i;
            lmp->left[i] = '\0';
            /* Right side */
            ln = strlen (lmp->right);
            rn = strlen (rmp->right);
            n = ln;
            if (n > rn)
              n = rn;
            for (i = 0; i < n; ++i)
              if (lmp->right[ln - i - 1] != rmp->right[rn - i - 1])
                break;
            for (j = 0; j < i; ++j)
              lmp->right[j] = lmp->right[(ln - i) + j];
            lmp->right[j] = '\0';
            new = inboth (lmp->in, rmp->in);
            freelist (lmp->in);
            free (lmp->in);
            lmp->in = new;
            freemust (rmp);
          }
          break;

        case PLUS:
          mp->is[0] = '\0';
          break;

        case END:
          assert (!mp->prev);
          for (i = 0; mp->in[i] != NULL; ++i)
            if (strlen (mp->in[i]) > strlen (result))
              result = mp->in[i];
          if (STREQ (result, mp->is))
            {
              exact = true;
              begline = mp->begline;
              endline = mp->endline;
            }
          goto done;

        case CAT:
          {
            must *rmp = mp;
            must *lmp = mp = mp->prev;

            /* In.  Everything in left, plus everything in
               right, plus concatenation of
               left's right and right's left.  */
            lmp->in = addlists (lmp->in, rmp->in);
            if (lmp->right[0] != '\0' && rmp->left[0] != '\0')
              {
                size_t lrlen = strlen (lmp->right);
                size_t rllen = strlen (rmp->left);
                char *tp = xmalloc (lrlen + rllen);
                memcpy (tp, lmp->right, lrlen);
                memcpy (tp + lrlen, rmp->left, rllen);
                lmp->in = enlist (lmp->in, tp, lrlen + rllen);
                free (tp);
              }
            /* Left-hand */
            if (lmp->is[0] != '\0')
              lmp->left = icatalloc (lmp->left, rmp->left);
            /* Right-hand */
            if (rmp->is[0] == '\0')
              lmp->right[0] = '\0';
            lmp->right = icatalloc (lmp->right, rmp->right);
            /* Guaranteed to be */
            if ((lmp->is[0] != '\0' || lmp->begline)
                && (rmp->is[0] != '\0' || rmp->endline))
              {
                lmp->is = icatalloc (lmp->is, rmp->is);
                lmp->endline = rmp->endline;
              }
            else
              {
                lmp->is[0] = '\0';
                lmp->begline = false;
                lmp->endline = false;
              }
            freemust (rmp);
          }
          break;

        case '\0':
          /* Not on *my* shift.  */
          goto done;

        default:
          mp = allocmust (mp);
          if (CSET <= t)
            {
              /* If T is a singleton, or if case-folding in a unibyte
                 locale and T's members all case-fold to the same char,
                 convert T to one of its members.  Otherwise, do
                 nothing further with T.  */
              charclass *ccl = &d->charclasses[t - CSET];
              int j;
              for (j = 0; j < NOTCHAR; j++)
                if (tstbit (j, *ccl))
                  break;
              if (! (j < NOTCHAR))
                break;
              t = j;
              while (++j < NOTCHAR)
                if (tstbit (j, *ccl)
                    && ! (case_fold && !d->multibyte
                          && toupper (j) == toupper (t)))
                  break;
              if (j < NOTCHAR)
                break;
            }
          mp->is[0] = mp->left[0] = mp->right[0]
            = case_fold && !d->multibyte ? toupper (t) : t;
          mp->is[1] = mp->left[1] = mp->right[1] = '\0';
          mp->in = enlist (mp->in, mp->is, 1);
          break;
        }
    }
done:
  if (*result)
    {
      dm = xmalloc (sizeof *dm);
      dm->exact = exact;
      dm->begline = begline;
      dm->endline = endline;
      dm->must = xstrdup (result);
      dm->next = d->musts;
      d->musts = dm;
    }

  while (mp)
    {
      must *prev = mp->prev;
      freemust (mp);
      mp = prev;
    }
}

struct dfa *
dfaalloc (void)
{
  return xmalloc (sizeof (struct dfa));
}

struct dfamust *_GL_ATTRIBUTE_PURE
dfamusts (struct dfa const *d)
{
  return d->musts;
}

/* vim:set shiftwidth=2: */