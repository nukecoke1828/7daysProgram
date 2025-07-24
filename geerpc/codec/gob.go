package codec

import (
	"bufio"        // 提供带缓冲的 I/O 操作
	"encoding/gob" // Go 自带的二进制编解码库
	"io"           // 定义基本的 I/O 接口
	"log"          // 日志输出
)

// 编译期断言：确保 *GobCodec 实现了 Codec 接口
var _ Codec = (*GobCodec)(nil)

// GobCodec 使用 gob 格式对 RPC 消息进行编解码的实现
type GobCodec struct {
	conn io.ReadWriteCloser // 底层连接（如 TCP 连接）
	buf  *bufio.Writer      // 带缓冲的写入器，减少系统调用次数
	dec  *gob.Decoder       // gob 解码器，从 conn 读取数据
	enc  *gob.Encoder       // gob 编码器，向 buf 写入数据
}

// NewGobCodec 构造并返回一个新的 GobCodec 实例
// 参数 conn 是用于通信的底层连接（如 net.Conn）
func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn) // 使用缓冲区包装连接，提高写入效率
	return &GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn), // 解码器直接从连接读取
		enc:  gob.NewEncoder(buf),  // 编码器写入缓冲区
	}
}

// ReadHeader 从连接中读取并解码消息头
// 参数 h 是指向 Header 的指针，用于存储读取到的数据
func (c *GobCodec) ReadHeader(h *Header) error {
	return c.dec.Decode(h)
}

// ReadBody 从连接中读取并解码消息体
// 参数 body 是一个 interface{}，用于接收解码后的数据
func (c *GobCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

// Write 将消息头和消息体编码后写入连接
// 使用缓冲区写入，最后统一 Flush，减少系统调用
func (c *GobCodec) Write(h *Header, body interface{}) (err error) {
	defer func() {
		// 确保缓冲区的数据被写入底层连接
		_ = c.buf.Flush()
		// 如果写入过程中发生错误，则关闭连接
		if err != nil {
			log.Fatal("rpc codec: gob error writing:", err)
			_ = c.conn.Close()
		}
	}()

	// 编码并写入消息头
	if err := c.enc.Encode(h); err != nil {
		log.Println("rpc codec: gob error encoding header:", err)
		return err
	}

	// 编码并写入消息体
	if err := c.enc.Encode(body); err != nil {
		log.Println("rpc codec: gob error encoding body:", err)
		return err
	}

	return nil
}

// Close 关闭底层连接
func (c *GobCodec) Close() error {
	return c.conn.Close()
}
