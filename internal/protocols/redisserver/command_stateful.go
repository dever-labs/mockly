package redisserver

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/redcon"
)

// statefulCommands is the bounded set of commands backed by the real
// in-memory dataStore when cfg.Mode == "stateful". Any command not in this
// set still falls through to the existing static-mock matcher, so existing
// configs keep working unchanged even in stateful mode.
var statefulCommands = map[string]bool{
	"SET": true, "GET": true, "DEL": true, "EXISTS": true,
	"EXPIRE": true, "TTL": true, "PERSIST": true,
	"INCR": true, "DECR": true, "INCRBY": true, "DECRBY": true, "APPEND": true,
	"HSET": true, "HGET": true, "HGETALL": true, "HDEL": true, "HEXISTS": true,
	"LPUSH": true, "RPUSH": true, "LRANGE": true, "LLEN": true,
}

// handleStateful executes command against the in-memory dataStore if it's
// one of statefulCommands, writing the RESP reply and returning true. It
// returns false (writing nothing) for any command outside that set, so the
// caller can fall back to static mock matching.
func (s *Server) handleStateful(conn redcon.Conn, command string, args [][]byte) bool {
	if !statefulCommands[command] {
		return false
	}

	argStrs := make([]string, len(args))
	for i, a := range args {
		argStrs[i] = string(a)
	}

	switch command {
	case "SET":
		s.cmdSet(conn, argStrs)
	case "GET":
		s.cmdGet(conn, argStrs)
	case "DEL":
		s.cmdDel(conn, argStrs)
	case "EXISTS":
		s.cmdExists(conn, argStrs)
	case "EXPIRE":
		s.cmdExpire(conn, argStrs)
	case "TTL":
		s.cmdTTL(conn, argStrs)
	case "PERSIST":
		s.cmdPersist(conn, argStrs)
	case "INCR":
		s.cmdIncrBy(conn, argStrs, 1, false)
	case "DECR":
		s.cmdIncrBy(conn, argStrs, -1, false)
	case "INCRBY":
		s.cmdIncrBy(conn, argStrs, 1, true)
	case "DECRBY":
		s.cmdIncrBy(conn, argStrs, -1, true)
	case "APPEND":
		s.cmdAppend(conn, argStrs)
	case "HSET":
		s.cmdHSet(conn, argStrs)
	case "HGET":
		s.cmdHGet(conn, argStrs)
	case "HGETALL":
		s.cmdHGetAll(conn, argStrs)
	case "HDEL":
		s.cmdHDel(conn, argStrs)
	case "HEXISTS":
		s.cmdHExists(conn, argStrs)
	case "LPUSH":
		s.cmdPush(conn, argStrs, true)
	case "RPUSH":
		s.cmdPush(conn, argStrs, false)
	case "LRANGE":
		s.cmdLRange(conn, argStrs)
	case "LLEN":
		s.cmdLLen(conn, argStrs)
	}
	return true
}

// wrongArgsErr writes the standard Redis "wrong number of arguments" error
// for the given (lowercase) command name.
func wrongArgsErr(conn redcon.Conn, command string) {
	conn.WriteError("ERR wrong number of arguments for '" + strings.ToLower(command) + "' command")
}

func writeDataErr(conn redcon.Conn, err error) {
	var wt wrongTypeErr
	if errors.As(err, &wt) {
		conn.WriteError(wt.Error())
		return
	}
	conn.WriteError("ERR value is not an integer or out of range")
}

func (s *Server) cmdSet(conn redcon.Conn, args []string) {
	if len(args) < 2 {
		wrongArgsErr(conn, "set")
		return
	}
	key, value := args[0], args[1]
	var ttl time.Duration
	for i := 2; i < len(args); i++ {
		switch strings.ToUpper(args[i]) {
		case "EX":
			if i+1 >= len(args) {
				wrongArgsErr(conn, "set")
				return
			}
			secs, err := strconv.ParseInt(args[i+1], 10, 64)
			if err != nil {
				conn.WriteError("ERR value is not an integer or out of range")
				return
			}
			ttl = time.Duration(secs) * time.Second
			i++
		case "PX":
			if i+1 >= len(args) {
				wrongArgsErr(conn, "set")
				return
			}
			ms, err := strconv.ParseInt(args[i+1], 10, 64)
			if err != nil {
				conn.WriteError("ERR value is not an integer or out of range")
				return
			}
			ttl = time.Duration(ms) * time.Millisecond
			i++
		}
	}
	s.data.Set(key, value, ttl)
	conn.WriteString("OK")
}

func (s *Server) cmdGet(conn redcon.Conn, args []string) {
	if len(args) < 1 {
		wrongArgsErr(conn, "get")
		return
	}
	v, ok, err := s.data.Get(args[0])
	if err != nil {
		writeDataErr(conn, err)
		return
	}
	if !ok {
		conn.WriteNull()
		return
	}
	conn.WriteBulkString(v)
}

func (s *Server) cmdDel(conn redcon.Conn, args []string) {
	if len(args) < 1 {
		wrongArgsErr(conn, "del")
		return
	}
	conn.WriteInt(s.data.Del(args...))
}

func (s *Server) cmdExists(conn redcon.Conn, args []string) {
	if len(args) < 1 {
		wrongArgsErr(conn, "exists")
		return
	}
	conn.WriteInt(s.data.Exists(args...))
}

func (s *Server) cmdExpire(conn redcon.Conn, args []string) {
	if len(args) != 2 {
		wrongArgsErr(conn, "expire")
		return
	}
	secs, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		conn.WriteError("ERR value is not an integer or out of range")
		return
	}
	if s.data.Expire(args[0], secs) {
		conn.WriteInt(1)
	} else {
		conn.WriteInt(0)
	}
}

func (s *Server) cmdTTL(conn redcon.Conn, args []string) {
	if len(args) != 1 {
		wrongArgsErr(conn, "ttl")
		return
	}
	conn.WriteInt64(s.data.TTL(args[0]))
}

func (s *Server) cmdPersist(conn redcon.Conn, args []string) {
	if len(args) != 1 {
		wrongArgsErr(conn, "persist")
		return
	}
	if s.data.Persist(args[0]) {
		conn.WriteInt(1)
	} else {
		conn.WriteInt(0)
	}
}

func (s *Server) cmdIncrBy(conn redcon.Conn, args []string, sign int64, explicit bool) {
	wantArgs := 1
	if explicit {
		wantArgs = 2
	}
	if len(args) != wantArgs {
		wrongArgsErr(conn, "incrby")
		return
	}
	delta := sign
	if explicit {
		n, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			conn.WriteError("ERR value is not an integer or out of range")
			return
		}
		delta = sign * n
	}
	n, err := s.data.IncrBy(args[0], delta)
	if err != nil {
		writeDataErr(conn, err)
		return
	}
	conn.WriteInt64(n)
}

func (s *Server) cmdAppend(conn redcon.Conn, args []string) {
	if len(args) != 2 {
		wrongArgsErr(conn, "append")
		return
	}
	n, err := s.data.Append(args[0], args[1])
	if err != nil {
		writeDataErr(conn, err)
		return
	}
	conn.WriteInt(n)
}

func (s *Server) cmdHSet(conn redcon.Conn, args []string) {
	if len(args) < 3 || len(args)%2 != 1 {
		wrongArgsErr(conn, "hset")
		return
	}
	fields := make(map[string]string, (len(args)-1)/2)
	for i := 1; i+1 < len(args); i += 2 {
		fields[args[i]] = args[i+1]
	}
	n, err := s.data.HSet(args[0], fields)
	if err != nil {
		writeDataErr(conn, err)
		return
	}
	conn.WriteInt(n)
}

func (s *Server) cmdHGet(conn redcon.Conn, args []string) {
	if len(args) != 2 {
		wrongArgsErr(conn, "hget")
		return
	}
	v, ok, err := s.data.HGet(args[0], args[1])
	if err != nil {
		writeDataErr(conn, err)
		return
	}
	if !ok {
		conn.WriteNull()
		return
	}
	conn.WriteBulkString(v)
}

func (s *Server) cmdHGetAll(conn redcon.Conn, args []string) {
	if len(args) != 1 {
		wrongArgsErr(conn, "hgetall")
		return
	}
	m, err := s.data.HGetAll(args[0])
	if err != nil {
		writeDataErr(conn, err)
		return
	}
	conn.WriteArray(len(m) * 2)
	for k, v := range m {
		conn.WriteBulkString(k)
		conn.WriteBulkString(v)
	}
}

func (s *Server) cmdHDel(conn redcon.Conn, args []string) {
	if len(args) < 2 {
		wrongArgsErr(conn, "hdel")
		return
	}
	n, err := s.data.HDel(args[0], args[1:]...)
	if err != nil {
		writeDataErr(conn, err)
		return
	}
	conn.WriteInt(n)
}

func (s *Server) cmdHExists(conn redcon.Conn, args []string) {
	if len(args) != 2 {
		wrongArgsErr(conn, "hexists")
		return
	}
	ok, err := s.data.HExists(args[0], args[1])
	if err != nil {
		writeDataErr(conn, err)
		return
	}
	if ok {
		conn.WriteInt(1)
	} else {
		conn.WriteInt(0)
	}
}

func (s *Server) cmdPush(conn redcon.Conn, args []string, left bool) {
	if len(args) < 2 {
		wrongArgsErr(conn, "lpush")
		return
	}
	var n int
	var err error
	if left {
		n, err = s.data.LPush(args[0], args[1:]...)
	} else {
		n, err = s.data.RPush(args[0], args[1:]...)
	}
	if err != nil {
		writeDataErr(conn, err)
		return
	}
	conn.WriteInt(n)
}

func (s *Server) cmdLRange(conn redcon.Conn, args []string) {
	if len(args) != 3 {
		wrongArgsErr(conn, "lrange")
		return
	}
	start, err1 := strconv.Atoi(args[1])
	stop, err2 := strconv.Atoi(args[2])
	if err1 != nil || err2 != nil {
		conn.WriteError("ERR value is not an integer or out of range")
		return
	}
	vals, err := s.data.LRange(args[0], start, stop)
	if err != nil {
		writeDataErr(conn, err)
		return
	}
	conn.WriteArray(len(vals))
	for _, v := range vals {
		conn.WriteBulkString(v)
	}
}

func (s *Server) cmdLLen(conn redcon.Conn, args []string) {
	if len(args) != 1 {
		wrongArgsErr(conn, "llen")
		return
	}
	n, err := s.data.LLen(args[0])
	if err != nil {
		writeDataErr(conn, err)
		return
	}
	conn.WriteInt(n)
}
