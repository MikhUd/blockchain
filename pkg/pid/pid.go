package pid

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"reflect"
	"time"
)

type PID struct {
	actor       Actor
	timestamp   int64
	extraFields map[string]FieldValue
}

type FieldValue any

type Actor byte

const (
	NoneActor Actor = iota
	NodeActor
	ClusterActor
)

const (
	IntFieldValue    = 0
	StringFieldValue = ""
	BoolFieldValue   = false
)

type TypeInt int
type TypeString string
type TypeBool bool

func New(actor Actor) *PID {
	p := &PID{
		actor:       actor,
		timestamp:   time.Now().Unix(),
		extraFields: make(map[string]FieldValue),
	}
	return p
}

func (p *PID) AddExtraField(key string, val any, t any) (*PID, error) {
	if reflect.TypeOf(val) != reflect.TypeOf(t) {
		return nil, errors.New("extra field type mismatch")
	}
	p.extraFields[key] = val
	return p, nil
}

func (p *PID) GetIntField(key string) (int, bool) {
	val, ok := p.extraFields[key].(TypeInt)
	return int(val), ok
}

func (p *PID) GetStringField(key string) (string, bool) {
	val, ok := p.extraFields[key].(TypeString)
	return string(val), ok
}

func (p *PID) GetBoolField(key string) (bool, bool) {
	val, ok := p.extraFields[key].(TypeBool)
	return bool(val), ok
}

func (p *PID) String() (string, error) {
	b, err := p.serialize()
	if err != nil {
		return "", err
	}
	return string(b[:]), nil
}

func (p *PID) FromString(pStr string) (*PID, error) {
	b := []byte(pStr)
	pid, err := p.deserialize(b)
	if err != nil {
		return nil, err
	}
	return pid, nil
}

func (p *PID) serialize() ([]byte, error) {
	var err error
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.LittleEndian, p.actor)
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, binary.LittleEndian, p.timestamp)
	if err != nil {
		return nil, err
	}
	for key, value := range p.extraFields {
		err = binary.Write(buf, binary.LittleEndian, int32(len(key)))
		if err != nil {
			return nil, err
		}
		err = binary.Write(buf, binary.LittleEndian, key)
		if err != nil {
			return nil, err
		}
		err = binary.Write(buf, binary.LittleEndian, value)
		if err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func (p *PID) deserialize(data []byte) (*PID, error) {
	pid := &PID{}
	buf := bytes.NewReader(data)
	if err := binary.Read(buf, binary.LittleEndian, &pid.actor); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &pid.timestamp); err != nil {
		return nil, err
	}
	for {
		var (
			key    string
			keyLen int32
		)
		if err := binary.Read(buf, binary.LittleEndian, &keyLen); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		keyBuf := make([]byte, keyLen)
		binary.Read(buf, binary.LittleEndian, &keyBuf)
		var value any
		if err := binary.Read(buf, binary.LittleEndian, &value); err != nil {
			return nil, err
		}
		pid.extraFields[key] = value
	}
	return pid, nil
}
