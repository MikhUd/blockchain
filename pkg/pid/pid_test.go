package pid

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPIDSerialize(t *testing.T) {
	pid := New(NoneActor)
	serialized, err := pid.serialize()
	require.NoError(t, err)
	deserialized, err := pid.deserialize(serialized)
	require.NoError(t, err)
	assert.Equal(t, pid.actor, deserialized.actor)
	assert.Equal(t, pid.timestamp, deserialized.timestamp)
}

func TestPIDGeneration(t *testing.T) {
	pid := New(NoneActor)
	serializedStr, err := pid.String()
	require.NoError(t, err)
	deserialized, err := pid.FromString(serializedStr)
	require.NoError(t, err)
	assert.Equal(t, pid.actor, deserialized.actor)
	assert.Equal(t, pid.timestamp, deserialized.timestamp)
}

func TestExtraFields(t *testing.T) {
	var (
		testNodeExtraFieldKey   = "id"
		testNodeExtraFieldValue = "test_node_id"
	)
	pid, err := New(NodeActor).AddExtraField(testNodeExtraFieldKey, testNodeExtraFieldValue, StringFieldValue)
	require.NoError(t, err)
	serializedStr, err := pid.String()
	require.NoError(t, err)
	deserialized, err := pid.FromString(serializedStr)
	require.NoError(t, err)
	assert.Equal(t, pid.actor, deserialized.actor)
	assert.Equal(t, pid.timestamp, deserialized.timestamp)
	val, ok := deserialized.GetStringField(testNodeExtraFieldKey)
	require.True(t, ok)
	assert.Equal(t, testNodeExtraFieldValue, val)
}
