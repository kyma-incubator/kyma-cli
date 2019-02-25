package step

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Step interface {
	Start()
	Status(msg string)
	Success()
	Successf(format string, args ...interface{})
	Failure()
	Failuref(format string, args ...interface{})
	Stop(success bool)
	Stopf(success bool, format string, args ...interface{})
	LogInfo(msg string)
	LogInfof(format string, args ...interface{})
	LogError(msg string)
	LogErrorf(format string, args ...interface{})
	Prompt(msg string) (string, error)
}

const (
	SuccessGliph  = "✅"
	FailureGliph  = "❌"
	WarningGliph  = "⚠️"
	QuestionGliph = "❓"
	InfoGliph     = "ℹ️"
)

func NewSimpleStep(msg string) Step {
	return &simpleStep{msg}
}

type simpleStep struct {
	msg string
}

func (s *simpleStep) Start() {
	fmt.Println(s.msg)
}

func (s *simpleStep) Status(msg string) {
	fmt.Printf("%s : %s\n", s.msg, msg)
}

func (s *simpleStep) Success() {
	s.Stop(true)
}

func (s *simpleStep) Successf(format string, args ...interface{}) {
	s.Stopf(true, format, args...)
}

func (s *simpleStep) Failure() {
	s.Stop(false)
}

func (s *simpleStep) Failuref(format string, args ...interface{}) {
	s.Stopf(false, format, args...)
}

func (s *simpleStep) Stopf(success bool, format string, args ...interface{}) {
	s.msg = fmt.Sprintf(format, args...)
	s.Stop(success)
}

func (s *simpleStep) Stop(success bool) {
	var glyph string
	if success {
		glyph = SuccessGliph
	} else {
		glyph = FailureGliph
	}
	fmt.Printf("%s %s\n", s.msg, glyph)
}

func (s *simpleStep) LogInfo(msg string) {
	fmt.Printf("%s %s\n", InfoGliph, msg)
}

func (s *simpleStep) LogInfof(format string, args ...interface{}) {
	s.LogInfo(fmt.Sprintf(format, args...))
}

func (s *simpleStep) LogError(msg string) {
	_, _ = fmt.Fprintln(os.Stderr, msg)
}

func (s *simpleStep) LogErrorf(format string, args ...interface{}) {
	s.LogError(fmt.Sprintf(format, args...))
}

func (s *simpleStep) Prompt(msg string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s %s", QuestionGliph, msg)
	answer, err := reader.ReadString('\n')
	return strings.TrimSpace(answer), err
}
