package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Counts struct {
	Lines, Words, Chars, Bytes, MaxLine, ContentChars int64
}

func getCounts(r io.Reader) (Counts, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Counts{}, err
	}

	lineReader := bytes.NewReader(data)
	scanner := bufio.NewScanner(lineReader)

	var lines int64
	for scanner.Scan() {
		lines++
	}

	if err := scanner.Err(); err != nil {
		return Counts{}, fmt.Errorf("scanner error: %w", err)
	}

	contentChars := int64(0)
	if len(data) > 0 {
		for _, runeValue := range string(data) { // range over runes (Unicode safe)
			if !unicode.IsSpace(runeValue) {
				contentChars++
			}
		}
	}

	return Counts{
		Lines:        lines,
		Words:        countWords(data),
		Chars:        int64(utf8.RuneCount(data)),
		Bytes:        int64(len(data)),
		MaxLine:      maxLineLength(data),
		ContentChars: contentChars,
	}, nil
}

func countWords(data []byte) int64 {
	var words int64
	inWord := false
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		data = data[size:]
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			inWord = true
			words++
		}
	}
	return words
}

func maxLineLength(data []byte) int64 {
	maxLen := int64(0)
	current := int64(0)
	for _, b := range data {
		current++
		if b == '\n' {
			if current-1 > maxLen {
				maxLen = current - 1
			}
			current = 0
		}
	}
	if current > maxLen {
		maxLen = current
	}
	return maxLen
}

func maxDigits(n int64) int {
	if n == 0 {
		return 1
	}
	d := 0
	for n > 0 {
		d++
		n /= 10
	}
	return d
}

func main() {
	fmt.Println("Hi this is first line of code")
	linesFlag := flag.Bool("l", false, "print newline counts")
	wordsFlag := flag.Bool("w", false, "print the word counts")
	charsFlag := flag.Bool("m", false, "print the character counts")
	bytesFlag := flag.Bool("c", false, "print the byte counts")
	maxLineFlag := flag.Bool("L", false, "print the maximum display width")
	contentFlag := flag.Bool("content", false, "count only non-whitespace characters (very useful for code size, git diff, project analysis)")
	files0From := flag.String("files0-from", "", "read input from the files specified by NUL-terminated names in file F")

	flag.Parse()

	showDefault := true

	if *linesFlag || *wordsFlag || *charsFlag || *bytesFlag || *maxLineFlag || *contentFlag {
		showDefault = false
	}

	if showDefault {
		*linesFlag = true
		*wordsFlag = true
		*bytesFlag = true
	}

	// if the user gave us a list file (--files0From) AND also typed filenames directly in the command line, stop and show an error.
	if *files0From != "" && len(flag.Args()) > 0 {
		fmt.Fprintln(os.Stderr, "wc: cannot specify both --files0-from and files")
		os.Exit(1)
	}

	var files []string
	if *files0From != "" {
		f := os.Stdin
		if *files0From != "-" {
			var err error
			f, err = os.Open(*files0From)
			if err != nil {
				fmt.Fprintf(os.Stderr, "wc: %v\n", err)
				os.Exit(1)
			}
			defer f.Close()
		}
		data, err := io.ReadAll(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wc: %v\n", err)
			os.Exit(1)
		}
		files = strings.Split(string(data), "\x00")
		if len(files) > 0 && files[len(files)-1] == "" {
			files = files[:len(files)-1]
		}
	} else if len(flag.Args()) == 0 {
		files = []string{"-"}
	} else {
		files = flag.Args()
	}

	type result struct {
		counts Counts
		name   string
		err    error
	}

	var results []result
	var totals Counts
	printTotal := len(files) > 1

	for _, f := range files {
		var r io.Reader
		name := f //store files name
		if f == "-" {
			r = os.Stdin // opens up stdin for reading in terminal
			name = ""
		} else {
			//checks for directory and if it is a directory, print error and continue to next file
			if fi, statErr := os.Stat(f); statErr == nil && fi.IsDir() {
				fmt.Fprintf(os.Stderr, "wc: %s: Is a directory\n", f)
				results = append(results, result{err: fmt.Errorf("is a directory"), name: f})
				continue
			}
			file, err := os.Open(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "wc: %s: %v\n", f, err)
				results = append(results, result{err: err, name: f})
				continue
			}
			defer file.Close()
			r = file
			name = f
		}

		c, err := getCounts(r)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wc: %s: %v\n", f, err)
			results = append(results, result{err: err, name: f})
			continue
		}

		results = append(results, result{counts: c, name: name})
		totals.Lines += c.Lines
		totals.Words += c.Words
		totals.Chars += c.Chars
		totals.Bytes += c.Bytes
		if c.MaxLine > totals.MaxLine {
			totals.MaxLine = c.MaxLine
		}
	}

	lineW, wordW, charW, byteW, maxLW, contentW := 1, 1, 1, 1, 1, 1

	for _, res := range results {
		if res.err != nil {
			continue
		}
		if *linesFlag {
			lineW = max(lineW, maxDigits(res.counts.Lines))
		}
		if *wordsFlag {
			wordW = max(wordW, maxDigits(res.counts.Words))
		}
		if *charsFlag {
			charW = max(charW, maxDigits(res.counts.Chars))
		}
		if *bytesFlag {
			byteW = max(byteW, maxDigits(res.counts.Bytes))
		}
		if *maxLineFlag {
			maxLW = max(maxLW, maxDigits(res.counts.MaxLine))
		}
		if *contentFlag {
			contentW = max(contentW, maxDigits(res.counts.ContentChars))
		}
	}

	// Also update for TOTAL line
	if printTotal && len(results) > 1 {
		if *contentFlag {
			contentW = max(contentW, maxDigits(totals.ContentChars))
		}
	}

	// Print helper
	printCounts := func(c Counts, name string) {
		sep := ""

		if *linesFlag {
			fmt.Printf("Lines : %s%*d ", sep, lineW, c.Lines)
			sep = " "
		}
		if *wordsFlag {
			fmt.Printf("Words : %s%*d ", sep, wordW, c.Words)
			sep = " "
		}
		if *charsFlag {
			fmt.Printf("Chars : %s%*d ", sep, charW, c.Chars)
			sep = " "
		}
		if *bytesFlag {
			fmt.Printf("Bytes : %s%*d ", sep, byteW, c.Bytes)
			sep = " "
		}
		if *maxLineFlag {
			fmt.Printf("MaxLine : %s%*d ", sep, maxLW, c.MaxLine)
			sep = " "
		}
		if *contentFlag { // ← THIS WAS MISSING
			fmt.Printf("ContentChars : %s%*d ", sep, contentW, c.ContentChars)
			sep = " "
		}

		if name != "" {
			fmt.Printf(" %s", name)
		}
		fmt.Println()
	}

	// Output results
	for _, res := range results {
		if res.err != nil {
			continue
		}
		printCounts(res.counts, res.name)
	}

	if printTotal && len(results) > 1 {
		printCounts(totals, "total")
	}
}
