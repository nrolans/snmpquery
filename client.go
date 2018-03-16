package snmpquery

import (
	"net"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/sleepinggenius2/gosmi/models"
	"github.com/sleepinggenius2/gosnmp"
)

// Client is a snmpquery client
type Client struct {
	snmp *gosnmp.GoSNMP
}

// Get is used to get the given scalar node formatted with the given format
func (c Client) Get(node models.ScalarNode, format ...models.Format) (val models.Value, err error) {
	oids := []string{node.OidFormatted}

	result, err := c.snmp.Get(oids)
	if err != nil {
		return val, errors.Wrap(err, "SNMP Get")
	}

	f := models.ResolveFormat(format)

	return node.Type.GetValueFormatter(f)(result.Variables[0].Value), nil
}

// Get is used to get the given column node with the given index formatted with the given format
func (c Client) GetIndex(node models.ColumnNode, index string, format ...models.Format) (val models.Value, err error) {
	oids := []string{node.OidFormatted + "." + index}

	result, err := c.snmp.Get(oids)
	if err != nil {
		return val, errors.Wrap(err, "SNMP Get")
	}

	f := models.ResolveFormat(format)

	return node.Type.GetValueFormatter(f)(result.Variables[0].Value), nil
}

// GetAll executes the given query
func (c Client) GetAll(q Query) (results map[string]models.Value, err error) {
	if len(q.Items) == 0 {
		return nil, errors.New("No items in query")
	}

	oids := make([]string, len(q.Items))

	for i, item := range q.Items {
		oids[i] = item.Oid
	}

	result, err := c.snmp.Get(oids)
	if err != nil {
		return nil, errors.Wrap(err, "SNMP Get")
	}

	results = make(map[string]models.Value, len(result.Variables))
	for i, variable := range result.Variables {
		results[q.Items[i].Name] = q.Items[i].Format(variable.Value)
	}

	return
}

// Connect is used to open a connection to the target
func (c Client) Connect() (err error) {
	return c.snmp.Connect()
}

// Close is used to close the connection to the target
func (c Client) Close() error {
	return c.snmp.Conn.Close()
}

func (c *Client) SetTimeout(d time.Duration) {
	c.snmp.Timeout = d
}

func newSNMP(target string) (*gosnmp.GoSNMP, error) {
	host, port, err := net.SplitHostPort(target)
	var portNum int
	if err != nil {
		if !strings.HasSuffix(err.Error(), "missing port in address") {
			return nil, err
		}
		host = target
		portNum = 161
	} else {
		portNum, err = net.LookupPort("udp", port)
		if err != nil {
			return nil, err
		}
	}
	return &gosnmp.GoSNMP{
		Target:  host,
		Port:    uint16(portNum),
		Timeout: 10 * time.Second,
		Retries: 3,
		MaxOids: gosnmp.MaxOids,
	}, nil
}

// NewV1 creates a new SNMPv1 Client
func NewV1(target, community string) (*Client, error) {
	snmp, err := newSNMP(target)
	if err != nil {
		return nil, err
	}
	snmp.Version = gosnmp.Version1
	snmp.Community = community
	return &Client{snmp: snmp}, nil
}

// NewV2 creates a new SNMPv2c Client
func NewV2(target, community string) (*Client, error) {
	snmp, err := newSNMP(target)
	if err != nil {
		return nil, err
	}
	snmp.Version = gosnmp.Version2c
	snmp.Community = community
	return &Client{snmp: snmp}, nil
}

func getAuth(password string) (protocol gosnmp.SnmpV3AuthProtocol, passphrase string, err error) {
	parts := strings.SplitN(password, ":", 2)

	if len(parts) == 2 {
		passphrase = parts[1]
	} else if parts[0] != "" {
		err = errors.New("Auth password given with no protocol")
		return
	}

	switch strings.ToLower(parts[0]) {
	case "md5":
		protocol = gosnmp.MD5
	case "sha":
		protocol = gosnmp.SHA
	case "":
		protocol = gosnmp.NoAuth
		if len(parts) == 1 {
			return
		}
		fallthrough
	default:
		err = errors.Errorf("Auth password given with invalid protocol: %q", parts[0])
	}
	return
}

func getPriv(password string) (protocol gosnmp.SnmpV3PrivProtocol, passphrase string, err error) {
	parts := strings.SplitN(password, ":", 2)

	if len(parts) == 2 {
		passphrase = parts[1]
	} else if parts[0] != "" {
		err = errors.New("Priv password given with no protocol")
		return
	}

	switch strings.ToLower(parts[0]) {
	case "aes":
		protocol = gosnmp.AES
	case "des":
		protocol = gosnmp.DES
	case "":
		protocol = gosnmp.NoPriv
		if len(parts) == 1 {
			return
		}
		fallthrough
	default:
		err = errors.Errorf("Priv password given with invalid protocol: %q", parts[0])
	}
	return
}

// NewV3 creates a mew SNMPv3 Client
func NewV3(target, username, authPassword, privPassword string) (*Client, error) {
	snmp, err := newSNMP(target)
	if err != nil {
		return nil, err
	}
	authProtocol, authPassphrase, err := getAuth(authPassword)
	if err != nil {
		return nil, err
	}
	privProtocol, privPassphrase, err := getPriv(privPassword)
	if err != nil {
		return nil, err
	}
	snmp.Version = gosnmp.Version3
	snmp.SecurityParameters = &gosnmp.UsmSecurityParameters{
		UserName:                 username,
		AuthenticationProtocol:   authProtocol,
		AuthenticationPassphrase: authPassphrase,
		PrivacyProtocol:          privProtocol,
		PrivacyPassphrase:        privPassphrase,
	}
	return &Client{snmp: snmp}, nil
}