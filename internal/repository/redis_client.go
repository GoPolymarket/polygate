package repository

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RedisClient struct {
	addr     string
	password string
	db       int

	mu     sync.Mutex
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

func NewRedisClient(addr, password string, db int) *RedisClient {
	return &RedisClient{
		addr:     strings.TrimSpace(addr),
		password: password,
		db:       db,
	}
}

func (c *RedisClient) Do(ctx context.Context, args ...string) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.addr == "" {
		return nil, fmt.Errorf("redis addr not configured")
	}
	if c.conn == nil {
		if err := c.connect(ctx); err != nil {
			return nil, err
		}
	}

	if err := c.writeCommand(args); err != nil {
		c.reset()
		return nil, err
	}
	resp, err := c.readResp()
	if err != nil {
		c.reset()
		return nil, err
	}
	return resp, nil
}

func (c *RedisClient) connect(ctx context.Context) error {
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return err
	}
	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.writer = bufio.NewWriter(conn)

	if c.password != "" {
		if _, err := c.doRaw(ctx, "AUTH", c.password); err != nil {
			c.reset()
			return err
		}
	}
	if c.db > 0 {
		if _, err := c.doRaw(ctx, "SELECT", strconv.Itoa(c.db)); err != nil {
			c.reset()
			return err
		}
	}
	return nil
}

func (c *RedisClient) doRaw(ctx context.Context, args ...string) (interface{}, error) {
	if err := c.writeCommand(args); err != nil {
		return nil, err
	}
	return c.readResp()
}

func (c *RedisClient) writeCommand(args []string) error {
	if c.writer == nil {
		return fmt.Errorf("redis connection not initialized")
	}
	if _, err := fmt.Fprintf(c.writer, "*%d\r\n", len(args)); err != nil {
		return err
	}
	for _, arg := range args {
		if _, err := fmt.Fprintf(c.writer, "$%d\r\n%s\r\n", len(arg), arg); err != nil {
			return err
		}
	}
	return c.writer.Flush()
}

func (c *RedisClient) readResp() (interface{}, error) {
	if c.reader == nil {
		return nil, fmt.Errorf("redis connection not initialized")
	}
	prefix, err := c.reader.ReadByte()
	if err != nil {
		return nil, err
	}
	switch prefix {
	case '+':
		line, err := c.readLine()
		return line, err
	case '-':
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("redis error: %s", line)
	case ':':
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		val, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return nil, err
		}
		return val, nil
	case '$':
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		n, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		if n == -1 {
			return nil, nil
		}
		buf := make([]byte, n+2)
		if _, err := c.reader.Read(buf); err != nil {
			return nil, err
		}
		return string(buf[:n]), nil
	case '*':
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		n, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		if n == -1 {
			return nil, nil
		}
		items := make([]interface{}, 0, n)
		for i := 0; i < n; i++ {
			val, err := c.readResp()
			if err != nil {
				return nil, err
			}
			items = append(items, val)
		}
		return items, nil
	default:
		return nil, fmt.Errorf("unknown redis response")
	}
}

func (c *RedisClient) readLine() (string, error) {
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSuffix(line, "\r\n")
	return line, nil
}

func (c *RedisClient) reset() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn = nil
	c.reader = nil
	c.writer = nil
}
