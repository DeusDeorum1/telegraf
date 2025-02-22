package tail

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/influxdata/telegraf/config"
)

func TestMultilineConfigOK(t *testing.T) {
	c := &multilineConfig{
		Pattern:        ".*",
		MatchWhichLine: previous,
	}

	_, err := c.newMultiline()

	require.NoError(t, err, "Configuration was OK.")
}

func TestMultilineConfigError(t *testing.T) {
	c := &multilineConfig{
		Pattern:        "\xA0",
		MatchWhichLine: previous,
	}

	_, err := c.newMultiline()

	require.Error(t, err, "The pattern was invalid")
}

func TestMultilineConfigTimeoutSpecified(t *testing.T) {
	duration := config.Duration(10 * time.Second)
	c := &multilineConfig{
		Pattern:        ".*",
		MatchWhichLine: previous,
		Timeout:        &duration,
	}
	m, err := c.newMultiline()
	require.NoError(t, err, "Configuration was OK.")

	require.Equal(t, duration, *m.config.Timeout)
}

func TestMultilineConfigDefaultTimeout(t *testing.T) {
	duration := config.Duration(5 * time.Second)
	c := &multilineConfig{
		Pattern:        ".*",
		MatchWhichLine: previous,
	}
	m, err := c.newMultiline()
	require.NoError(t, err, "Configuration was OK.")

	require.Equal(t, duration, *m.config.Timeout)
}

func TestMultilineIsEnabled(t *testing.T) {
	c := &multilineConfig{
		Pattern:        ".*",
		MatchWhichLine: previous,
	}
	m, err := c.newMultiline()
	require.NoError(t, err, "Configuration was OK.")

	isEnabled := m.isEnabled()

	require.True(t, isEnabled, "Should have been enabled")
}

func TestMultilineIsDisabled(t *testing.T) {
	c := &multilineConfig{
		MatchWhichLine: previous,
	}
	m, err := c.newMultiline()
	require.NoError(t, err, "Configuration was OK.")

	isEnabled := m.isEnabled()

	require.False(t, isEnabled, "Should have been disabled")
}

func TestMultilineFlushEmpty(t *testing.T) {
	var buffer bytes.Buffer
	text := flush(&buffer)

	require.Empty(t, text)
}

func TestMultilineFlush(t *testing.T) {
	var buffer bytes.Buffer
	buffer.WriteString("foo")

	text := flush(&buffer)
	require.Equal(t, "foo", text)
	require.Zero(t, buffer.Len())
}

func TestMultiLineProcessLinePrevious(t *testing.T) {
	c := &multilineConfig{
		Pattern:        "^=>",
		MatchWhichLine: previous,
	}
	m, err := c.newMultiline()
	require.NoError(t, err, "Configuration was OK.")
	var buffer bytes.Buffer

	text := m.processLine("1", &buffer)
	require.Empty(t, text)
	require.NotZero(t, buffer.Len())

	text = m.processLine("=>2", &buffer)
	require.Empty(t, text)
	require.NotZero(t, buffer.Len())

	text = m.processLine("=>3", &buffer)
	require.Empty(t, text)
	require.NotZero(t, buffer.Len())

	text = m.processLine("4", &buffer)
	require.Equal(t, "1=>2=>3", text)
	require.NotZero(t, buffer.Len())

	text = m.processLine("5", &buffer)
	require.Equal(t, "4", text)
	require.Equal(t, "5", buffer.String())
}

func TestMultiLineProcessLineNext(t *testing.T) {
	c := &multilineConfig{
		Pattern:        "=>$",
		MatchWhichLine: next,
	}
	m, err := c.newMultiline()
	require.NoError(t, err, "Configuration was OK.")
	var buffer bytes.Buffer

	text := m.processLine("1=>", &buffer)
	require.Empty(t, text)
	require.NotZero(t, buffer.Len())

	text = m.processLine("2=>", &buffer)
	require.Empty(t, text)
	require.NotZero(t, buffer.Len())

	text = m.processLine("3=>", &buffer)
	require.Empty(t, text)
	require.NotZero(t, buffer.Len())

	text = m.processLine("4", &buffer)
	require.Equal(t, "1=>2=>3=>4", text)
	require.Zero(t, buffer.Len())

	text = m.processLine("5", &buffer)
	require.Equal(t, "5", text)
	require.Zero(t, buffer.Len())
}

func TestMultiLineMatchStringWithInvertMatchFalse(t *testing.T) {
	c := &multilineConfig{
		Pattern:        "=>$",
		MatchWhichLine: next,
		InvertMatch:    false,
	}
	m, err := c.newMultiline()
	require.NoError(t, err, "Configuration was OK.")

	matches1 := m.matchString("t=>")
	matches2 := m.matchString("t")

	require.True(t, matches1)
	require.False(t, matches2)
}

func TestMultiLineMatchStringWithInvertTrue(t *testing.T) {
	c := &multilineConfig{
		Pattern:        "=>$",
		MatchWhichLine: next,
		InvertMatch:    true,
	}
	m, err := c.newMultiline()
	require.NoError(t, err, "Configuration was OK.")

	matches1 := m.matchString("t=>")
	matches2 := m.matchString("t")

	require.False(t, matches1)
	require.True(t, matches2)
}

func TestMultilineWhat(t *testing.T) {
	var w1 multilineMatchWhichLine
	require.NoError(t, w1.UnmarshalTOML([]byte(`"previous"`)))
	require.Equal(t, previous, w1)

	var w2 multilineMatchWhichLine
	require.NoError(t, w2.UnmarshalTOML([]byte(`previous`)))
	require.Equal(t, previous, w2)

	var w3 multilineMatchWhichLine
	require.NoError(t, w3.UnmarshalTOML([]byte(`'previous'`)))
	require.Equal(t, previous, w3)

	var w4 multilineMatchWhichLine
	require.NoError(t, w4.UnmarshalTOML([]byte(`"next"`)))
	require.Equal(t, next, w4)

	var w5 multilineMatchWhichLine
	require.NoError(t, w5.UnmarshalTOML([]byte(`next`)))
	require.Equal(t, next, w5)

	var w6 multilineMatchWhichLine
	require.NoError(t, w6.UnmarshalTOML([]byte(`'next'`)))
	require.Equal(t, next, w6)

	var w7 multilineMatchWhichLine
	require.Error(t, w7.UnmarshalTOML([]byte(`nope`)))
	require.Equal(t, multilineMatchWhichLine(-1), w7)
}

func TestMultilineQuoted(t *testing.T) {
	tests := []struct {
		name      string
		quotation string
		quote     string
		filename  string
	}{
		{
			name:      "single-quotes",
			quotation: "single-quotes",
			quote:     `'`,
			filename:  "multiline_quoted_single.csv",
		},
		{
			name:      "double-quotes",
			quotation: "double-quotes",
			quote:     `"`,
			filename:  "multiline_quoted_double.csv",
		},
		{
			name:      "backticks",
			quotation: "backticks",
			quote:     "`",
			filename:  "multiline_quoted_backticks.csv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := []string{
				`1660819827410,1,some text without quotes,A`,
				fmt.Sprintf("1660819827411,1,%ssome text all quoted%s,A", tt.quote, tt.quote),
				fmt.Sprintf("1660819827412,1,%ssome text all quoted\nbut wrapped%s,A", tt.quote, tt.quote),
				fmt.Sprintf("1660819827420,2,some text with %squotes%s,B", tt.quote, tt.quote),
				"1660819827430,3,some text with 'multiple \"quotes\" in `one` line',C",
				fmt.Sprintf("1660819827440,4,some multiline text with %squotes\n", tt.quote) +
					fmt.Sprintf("spanning \\%smultiple\\%s\n", tt.quote, tt.quote) +
					fmt.Sprintf("lines%s but do not %send\ndirectly%s,D", tt.quote, tt.quote, tt.quote),
				fmt.Sprintf("1660819827450,5,all of %sthis%s should %sbasically%s work...,E", tt.quote, tt.quote, tt.quote, tt.quote),
			}

			c := &multilineConfig{
				MatchWhichLine:  next,
				Quotation:       tt.quotation,
				PreserveNewline: true,
			}
			m, err := c.newMultiline()
			require.NoError(t, err)

			f, err := os.Open(filepath.Join("testdata", tt.filename))
			require.NoError(t, err)

			scanner := bufio.NewScanner(f)

			var buffer bytes.Buffer
			var result []string
			for scanner.Scan() {
				line := scanner.Text()

				text := m.processLine(line, &buffer)
				if text == "" {
					continue
				}
				result = append(result, text)
			}
			if text := flush(&buffer); text != "" {
				result = append(result, text)
			}

			require.EqualValues(t, expected, result)
		})
	}
}

func TestMultilineQuotedError(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		quotation string
		quote     string
		expected  []string
	}{
		{
			name:      "messed up quoting",
			filename:  "multiline_quoted_messed_up.csv",
			quotation: "single-quotes",
			quote:     `'`,
			expected: []string{
				"1660819827410,1,some text without quotes,A",
				"1660819827411,1,'some text all quoted,A\n1660819827412,1,'some text all quoted",
				"but wrapped,A"},
		},
		{
			name:      "missing closing quote",
			filename:  "multiline_quoted_missing_close.csv",
			quotation: "single-quotes",
			quote:     `'`,
			expected:  []string{"1660819827411,2,'some text all quoted,B\n1660819827410,1,some text without quotes,A"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &multilineConfig{
				MatchWhichLine:  next,
				Quotation:       tt.quotation,
				PreserveNewline: true,
			}
			m, err := c.newMultiline()
			require.NoError(t, err)

			f, err := os.Open(filepath.Join("testdata", tt.filename))
			require.NoError(t, err)

			scanner := bufio.NewScanner(f)

			var buffer bytes.Buffer
			var result []string
			for scanner.Scan() {
				line := scanner.Text()

				text := m.processLine(line, &buffer)
				if text == "" {
					continue
				}
				result = append(result, text)
			}
			if text := flush(&buffer); text != "" {
				result = append(result, text)
			}

			require.EqualValues(t, tt.expected, result)
		})
	}
}

func TestMultilineNewline(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		cfg      *multilineConfig
		expected []string
	}{
		{
			name: "do not preserve newline",
			cfg: &multilineConfig{
				Pattern:     `\[[0-9]{2}/[A-Za-z]{3}/[0-9]{4}:[0-9]{2}:[0-9]{2}:[0-9]{2} \+[0-9]{4}\]`,
				InvertMatch: true,
			},
			filename: "test_multiline.log",
			expected: []string{
				`[04/Jun/2016:12:41:45 +0100] DEBUG HelloExample: This is debug`,
				`[04/Jun/2016:12:41:48 +0100] INFO HelloExample: This is info`,
				"[04/Jun/2016:12:41:46 +0100] ERROR HelloExample: Sorry, something wrong! " +
					"java.lang.ArithmeticException: / by zero" +
					"\tat com.foo.HelloExample2.divide(HelloExample2.java:24)" +
					"\tat com.foo.HelloExample2.main(HelloExample2.java:14)",
				`[04/Jun/2016:12:41:48 +0100] WARN HelloExample: This is warn`,
			},
		},
		{
			name: "preserve newline",
			cfg: &multilineConfig{
				Pattern:         `\[[0-9]{2}/[A-Za-z]{3}/[0-9]{4}:[0-9]{2}:[0-9]{2}:[0-9]{2} \+[0-9]{4}\]`,
				InvertMatch:     true,
				PreserveNewline: true,
			},
			filename: "test_multiline.log",
			expected: []string{
				`[04/Jun/2016:12:41:45 +0100] DEBUG HelloExample: This is debug`,
				`[04/Jun/2016:12:41:48 +0100] INFO HelloExample: This is info`,
				`[04/Jun/2016:12:41:46 +0100] ERROR HelloExample: Sorry, something wrong!` + ` ` + `
java.lang.ArithmeticException: / by zero
	at com.foo.HelloExample2.divide(HelloExample2.java:24)
	at com.foo.HelloExample2.main(HelloExample2.java:14)`,
				`[04/Jun/2016:12:41:48 +0100] WARN HelloExample: This is warn`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := tt.cfg.newMultiline()
			require.NoError(t, err)

			f, err := os.Open(filepath.Join("testdata", tt.filename))
			require.NoError(t, err)

			scanner := bufio.NewScanner(f)

			var buffer bytes.Buffer
			var result []string
			for scanner.Scan() {
				line := scanner.Text()

				text := m.processLine(line, &buffer)
				if text == "" {
					continue
				}
				result = append(result, text)
			}
			if text := flush(&buffer); text != "" {
				result = append(result, text)
			}

			require.EqualValues(t, tt.expected, result)
		})
	}
}

func TestMultiLineQuotedAndPattern(t *testing.T) {
	c := &multilineConfig{
		Pattern:         "=>$",
		MatchWhichLine:  next,
		Quotation:       "double-quotes",
		PreserveNewline: true,
	}
	m, err := c.newMultiline()
	require.NoError(t, err, "Configuration was OK.")
	var buffer bytes.Buffer

	text := m.processLine("1=>", &buffer)
	require.Empty(t, text)
	require.NotZero(t, buffer.Len())

	text = m.processLine("2=>", &buffer)
	require.Empty(t, text)
	require.NotZero(t, buffer.Len())

	text = m.processLine(`"a quoted`, &buffer)
	require.Empty(t, text)
	require.NotZero(t, buffer.Len())

	text = m.processLine(`multiline string"=>`, &buffer)
	require.Empty(t, text)
	require.NotZero(t, buffer.Len())

	text = m.processLine("3=>", &buffer)
	require.Empty(t, text)
	require.NotZero(t, buffer.Len())

	text = m.processLine("4", &buffer)
	require.Equal(t, "1=>\n2=>\n\"a quoted\nmultiline string\"=>\n3=>\n4", text)
	require.Zero(t, buffer.Len())

	text = m.processLine("5", &buffer)
	require.Equal(t, "5", text)
	require.Zero(t, buffer.Len())
}
