package serializer

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type Serializer interface {
	Serialize(msg any) ([]byte, error)
	TypeName(any) string
}

type Deserializer interface {
	Deserialize([]byte, string) (any, error)
}

type ProtoSerializer struct{}

func (ProtoSerializer) Deserialize(data []byte, typeName string) (any, error) {
	protoName := protoreflect.FullName(typeName)
	n, err := protoregistry.GlobalTypes.FindMessageByName(protoName)
	if err != nil {
		return nil, err
	}
	protoMessage := n.New().Interface()
	err = proto.Unmarshal(data, protoMessage)
	return protoMessage, err
}

func (ProtoSerializer) Serialize(msg any) ([]byte, error) {
	return proto.Marshal(msg.(proto.Message))
}

func (ProtoSerializer) TypeName(msg any) string {
	return string(proto.MessageName(msg.(proto.Message)))
}
