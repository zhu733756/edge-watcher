package ipset

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAndSet(t *testing.T) {
	req := require.New(t)
	testCase := []string{"192.168.0.1", "192.168.0.2"}
	m := NewIpset("default")
	getFlag := m.Get(testCase[0])
	req.Equal(getFlag, false)
	setFlag := m.Set(testCase[0])
	req.Equal(setFlag, true)
	getFlag = m.Get(testCase[0])
	req.Equal(getFlag, true)
}

func TestGetMany(t *testing.T) {
	req := require.New(t)
	testCase := []string{"192.168.0.1", "192.168.0.2"}
	m := NewIpset("default")
	getFlag := m.GetMany(testCase...)
	req.Equal(getFlag, false)
}

func TestSetMany(t *testing.T) {
	req := require.New(t)
	testCase := []string{"192.168.0.1", "192.168.0.2"}
	m := NewIpset("default")
	setFlag := m.SetMany(testCase...)
	req.Equal(setFlag, true)
}

func TestSetOrGET(t *testing.T) {
	req := require.New(t)
	testCase := []string{"192.168.0.1", "192.168.0.2"}
	m := NewIpset("default")
	f := m.SetOrGet(testCase[0])
	req.Equal(f, true)
	f = m.SetOrGet(testCase[0])
	req.Equal(f, false)
	f = m.SetOrGet(testCase[1])
	req.Equal(f, true)
	f = m.SetOrGet(testCase[1])
	req.Equal(f, false)
}

func TestSetManyOrGETMany(t *testing.T) {
	req := require.New(t)
	testCase := []string{"192.168.0.1", "192.168.0.2"}
	m := NewIpset("default")
	f := m.SetManyOrGetMany(testCase...)
	req.Equal(f, true)
	f = m.SetManyOrGetMany(testCase...)
	req.Equal(f, false)
}

func TestSetManyAndGetOne(t *testing.T) {
	req := require.New(t)
	testCase := []string{"192.168.0.1", "192.168.0.2"}
	m := NewIpset("default")
	setFlag := m.SetMany(testCase...)
	req.Equal(setFlag, true)
	getFlag := m.Get(testCase[0])
	req.Equal(getFlag, true)
	getFlag = m.Get(testCase[1])
	req.Equal(getFlag, true)
}

func TestSetOneAndGetMany(t *testing.T) {
	req := require.New(t)
	testCase := []string{"192.168.0.1", "192.168.0.2"}
	m := NewIpset("default")
	setFlag := m.Set(testCase[0])
	req.Equal(setFlag, true)
	setFlag = m.Set(testCase[1])
	req.Equal(setFlag, true)
	getFlag := m.GetMany(testCase...)
	req.Equal(getFlag, true)
}

func TestSetManyAndGetMany(t *testing.T) {
	req := require.New(t)
	testCase := []string{"192.168.0.1", "192.168.0.2"}
	m := NewIpset("default")
	getFlag := m.GetMany(testCase...)
	req.Equal(getFlag, false)
	setFlag := m.SetMany(testCase...)
	req.Equal(setFlag, true)
	getFlag = m.Get(testCase[0])
	req.Equal(getFlag, true)
}

func TestNotInArpTable(t *testing.T) {
	req := require.New(t)
	m := NewIpset("default")
	testCase := []string{"192.16.0.1", "192.16.0.2"}
	f := m.NotInArpTable(testCase...)
	req.Equal(f, true)
}
