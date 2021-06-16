package fwncs

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/n-creativesystem/go-fwncs/constant"
	"github.com/valyala/fasttemplate"
)

const FormatTemplate = "time:${time_millisec}\tid:${id}\tremote_ip:${remote_ip}\thost:${host}\tmethod:${method}\turi:${uri}\tuser_agent:${user_agent}\t" +
	"status:${status}\terror:${error}\tlatency:${latency}\tlatency_human:${latency_human}\tbytes_in:${bytes_in}\tbytes_out:${bytes_out}"

type LoggerConfig struct {
	Format           string
	CustomTimeFormat string
	CustomMessage    func(tag string, writer io.Writer)
	template         *fasttemplate.Template
	pool             *sync.Pool
}

var DefaultLoggerConfig = LoggerConfig{
	Format:           FormatTemplate,
	CustomTimeFormat: FormatMillisec.String(),
}

func Logger() HandlerFunc {
	return LoggerWithConfig(DefaultLoggerConfig)
}

func LoggerWithConfig(config LoggerConfig) HandlerFunc {
	if config.Format == "" {
		config.Format = DefaultLoggerConfig.Format
	}
	if config.CustomTimeFormat == "" {
		config.CustomTimeFormat = DefaultLoggerConfig.CustomTimeFormat
	}
	config.template = fasttemplate.New(config.Format, "${", "}")
	config.pool = &sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 256))
		},
	}
	return func(c Context) {
		req := c.Request()
		start := time.Now()
		c.Next()
		stop := time.Now()
		buf := config.pool.Get().(*bytes.Buffer)
		buf.Reset()
		defer config.pool.Put(buf)
		now := time.Now()
		if _, err := config.template.ExecuteFunc(buf, func(w io.Writer, tag string) (int, error) {
			switch tag {
			case "time_unix":
				return buf.WriteString(strconv.FormatInt(now.Unix(), 10))
			case "time_unix_nano":
				return buf.WriteString(strconv.FormatInt(now.UnixNano(), 10))
			case "time_rfc3339":
				return buf.WriteString(now.Format(time.RFC3339))
			case "time_rfc3339_nano":
				return buf.WriteString(now.Format(time.RFC3339Nano))
			case "time_date":
				return buf.WriteString(now.Format(FormatDate.String()))
			case "time_datetime":
				return buf.WriteString(now.Format(FormatDatetime.String()))
			case "time_millisec":
				return buf.WriteString(now.Format(FormatMillisec.String()))
			case "time_custom":
				return buf.WriteString(now.Format(config.CustomTimeFormat))
			case "id":
				id := req.Header.Get(constant.HeaderXRequestID)
				if id == "" {
					id = c.Writer().Header().Get(constant.HeaderXRequestID)
				}
				return buf.WriteString(id)
			case "remote_ip":
				return buf.WriteString(c.ClientIP())
			case "host":
				return buf.WriteString(req.Host)
			case "uri":
				return buf.WriteString(req.RequestURI)
			case "method":
				return buf.WriteString(req.Method)
			case "path":
				p := req.URL.Path
				if p == "" {
					p = "/"
				}
				return buf.WriteString(p)
			case "protocol":
				return buf.WriteString(req.Proto)
			case "referer":
				return buf.WriteString(req.Referer())
			case "user_agent":
				return buf.WriteString(req.UserAgent())
			case "status":
				return buf.WriteString(strconv.Itoa(c.GetStatus()))
			case "error":
				if c.GetError() != nil {
					errs := c.GetError()
					b, _ := json.Marshal(errs)
					b = b[1 : len(b)-1]
					return buf.Write(b)
				}
			case "latency":
				l := stop.Sub(start)
				return buf.WriteString(strconv.FormatInt(int64(l), 10))
			case "latency_human":
				return buf.WriteString(stop.Sub(start).String())
			case "bytes_in":
				cl := req.Header.Get(constant.HeaderContentLength)
				if cl == "" {
					cl = "0"
				}
				return buf.WriteString(cl)
			case "bytes_out":
				return buf.WriteString(strconv.FormatInt(int64(c.Writer().Size()), 10))
			default:
				switch {
				case strings.HasPrefix(tag, "header:"):
					return buf.Write([]byte(c.Request().Header.Get(tag[7:])))
				case strings.HasPrefix(tag, "query:"):
					return buf.Write([]byte(c.QueryParam(tag[6:])))
				case strings.HasPrefix(tag, "form:"):
					return buf.Write([]byte(c.FormValue(tag[5:])))
				case strings.HasPrefix(tag, "cookie:"):
					cookie, err := c.Cookie(tag[7:])
					if err == nil {
						return buf.Write([]byte(cookie.Value))
					}
				default:
					if config.CustomMessage != nil {
						config.CustomMessage(tag, buf)
					}
				}
			}
			return 0, nil
		}); err != nil {
			return
		}
		c.Logger().Info(buf.String())
	}
}
