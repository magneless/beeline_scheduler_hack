package optimized

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func (o *Optimized) executable() (string, error) {
	binary := o.binary
	if binary == "" {
		binary = "vroom"
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return "", computationError("Не найден исполняемый файл VROOM; установите VROOM 1.15 или задайте VROOM_BIN (см. backend/README.md)")
	}
	return path, nil
}

func runVROOM(ctx context.Context, binary string, input vroomInput, deadline time.Time) ([]byte, error) {
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	body, err := json.Marshal(input)
	if err != nil {
		return nil, computationError(fmt.Sprintf("Подготовка запроса VROOM: %v", err))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	remaining := time.Until(deadline)
	// The native limit covers search, whereas the context also covers process
	// startup, input parsing and output. Leave time for returning its incumbent.
	limit := max(time.Millisecond, remaining-min(10*time.Millisecond, remaining/5))
	cmd := exec.CommandContext(ctx, binary, "-t", "2", "-x", "5", "-l", strconv.FormatFloat(limit.Seconds(), 'f', 3, 64))
	cmd.Stdin = bytes.NewReader(body)
	stdout := cappedBuffer{limit: 32 << 20}
	stderr := cappedBuffer{limit: 16 << 10}
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = 100 * time.Millisecond
	err = cmd.Run() // CommandContext kills and Wait reaps this call's process.
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if len(message) > 2048 {
			message = message[:2048]
		}
		return nil, computationError(fmt.Sprintf("Ошибка запуска VROOM: %v: %s", err, message))
	}
	return stdout.Bytes(), nil
}

type cappedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *cappedBuffer) Write(data []byte) (int, error) {
	if len(data) > b.limit-b.Len() {
		return 0, fmt.Errorf("VROOM output exceeds %d bytes", b.limit)
	}
	return b.Buffer.Write(data)
}
