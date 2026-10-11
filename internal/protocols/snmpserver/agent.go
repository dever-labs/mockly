package snmpserver

import (
	"bytes"
	"crypto/hmac"
	"crypto/subtle"
	"fmt"
	"io"
	"log"

	"github.com/gosnmp/gosnmp"
)

var discardLogger = gosnmp.NewLogger(log.New(io.Discard, "", 0))

// pduItem describes a single OID's get/set behavior. It plays the same role
// as GoSNMPServer's PDUValueControlItem, simplified to the subset mockly
// actually uses: every mock always provides OnGet, and OnSet is set only for
// writable mocks. GoSNMPServer's OnTrap/OnCheckPermission/NonWalkable are
// never used by mockly's own buildOIDs and so have no equivalent here.
type pduItem struct {
	oid      string
	typ      gosnmp.Asn1BER
	document string
	onGet    func() (interface{}, error)
	onSet    func(value interface{}) error
}

// agent implements the SNMP agent request/response logic that
// github.com/slayercat/GoSNMPServer used to provide for mockly: SNMPv1/v2c
// community checking, SNMPv3 USM authentication (including the engine-ID
// discovery handshake), and Get/GetNext/GetBulk/Set dispatch against a
// sorted OID table. All ASN.1/BER wire encoding/decoding and SNMPv3 USM
// crypto is delegated to github.com/gosnmp/gosnmp, which mockly already
// depends on directly for its SNMP client use (trap sending); this type only
// owns the agent-side business logic layered on top of it.
//
// This replaces GoSNMPServer, which has an unfixed data race in its UDP
// listener and no commits addressing it since its last release — see issue
// #113.
type agent struct {
	community   string
	snmpV3Only  bool
	users       []gosnmp.UsmSecurityParameters
	engineID    string
	engineBoots uint32
	engineTime  func() uint32

	oids []*pduItem // kept sorted by numeric OID order; see sortOIDs
}

// newAgent builds an agent, sorting items by OID as GoSNMPServer's
// SubAgent.SyncConfig used to.
func newAgent(community string, snmpV3Only bool, users []gosnmp.UsmSecurityParameters, engineID string, engineTime func() uint32, items []*pduItem) *agent {
	sortOIDs(items)
	return &agent{
		community:   community,
		snmpV3Only:  snmpV3Only,
		users:       users,
		engineID:    engineID,
		engineBoots: 1,
		engineTime:  engineTime,
		oids:        items,
	}
}

// respond decodes a raw incoming UDP payload, dispatches it against the OID
// table, and returns the raw response bytes to write back. A nil []byte with
// a nil error means no response should be sent at all — mirroring common
// real-world SNMP agent behavior of silently dropping requests with
// unrecognised credentials (wrong community / unknown v3 user / wrong v3
// auth mode) rather than confirming which credentials are valid.
func (a *agent) respond(buf []byte) ([]byte, error) {
	probe := gosnmp.GoSNMP{Logger: discardLogger}
	probe.SecurityParameters = a.blankUSM()
	req, decodeErr := probe.SnmpDecodePacket(buf)
	if req == nil {
		return nil, fmt.Errorf("decode snmp packet: %w", decodeErr)
	}

	switch req.Version {
	case gosnmp.Version1, gosnmp.Version2c:
		return a.respondCommunity(req)
	case gosnmp.Version3:
		return a.respondV3(req, buf, decodeErr)
	default:
		return nil, fmt.Errorf("unsupported snmp version %v", req.Version)
	}
}

func (a *agent) respondCommunity(req *gosnmp.SnmpPacket) ([]byte, error) {
	if a.snmpV3Only || req.Community != a.community {
		return nil, nil
	}
	resp, err := a.serve(req)
	if err != nil {
		return nil, nil //nolint:nilerr // unsupported/unknown PDU: drop silently, not a transport error
	}
	return resp.MarshalMsg()
}

func (a *agent) respondV3(req *gosnmp.SnmpPacket, raw []byte, decodeErr error) ([]byte, error) {
	if decodeErr == nil && len(req.Variables) == 0 {
		// Initial "hello"/discovery packet: reply with a Report PDU carrying
		// usmStatsUnknownEngineIDs so the client learns our engine
		// ID/boots/time and retries with full authentication. Real SNMPv3
		// clients (including gosnmp's) perform this handshake automatically
		// before ever sending an authenticated request.
		resp := &gosnmp.SnmpPacket{
			Version:            gosnmp.Version3,
			MsgID:              req.MsgID,
			MsgMaxSize:         req.MsgMaxSize,
			SecurityModel:      req.SecurityModel,
			SecurityParameters: a.blankUSM(),
			ContextEngineID:    req.ContextEngineID,
			ContextName:        req.ContextName,
			PDUType:            gosnmp.Report,
			RequestID:          req.RequestID,
			Variables:          []gosnmp.SnmpPDU{helloVariable()},
		}
		return resp.MarshalMsg()
	}

	if req.SecurityParameters == nil {
		return nil, fmt.Errorf("decode v3 packet: %w", decodeErr)
	}
	usp, ok := req.SecurityParameters.(*gosnmp.UsmSecurityParameters)
	if !ok {
		return nil, fmt.Errorf("unsupported snmpv3 security model")
	}
	user := a.findUser(usp.UserName)
	if user == nil {
		return nil, nil
	}

	wantAuth := gosnmp.NoAuthNoPriv
	if user.AuthenticationProtocol > gosnmp.NoAuth {
		wantAuth = gosnmp.AuthNoPriv
	}
	if user.PrivacyProtocol > gosnmp.NoPriv {
		wantAuth = gosnmp.AuthPriv
	}
	if req.MsgFlags&gosnmp.AuthPriv != gosnmp.SnmpV3MsgFlags(wantAuth) {
		return nil, nil
	}

	// Localize the user's auth/priv keys against our real engine ID. The
	// copy cached on a.users was localized in buildUsers() against an empty
	// engine ID (used only to validate the passphrase up front), so it must
	// be recomputed here before it can verify or sign anything.
	localUser := *user //nolint:govet // freshly derived value, embedded mutex never locked; see buildUsers
	localUser.AuthoritativeEngineID = a.engineID
	localUser.AuthoritativeEngineBoots = a.engineBoots
	localUser.AuthoritativeEngineTime = a.engineTime()
	localUser.SecretKey = nil
	localUser.PrivacyKey = nil
	if err := localUser.InitSecurityKeys(); err != nil {
		return nil, fmt.Errorf("init security keys: %w", err)
	}

	if user.AuthenticationProtocol > gosnmp.NoAuth &&
		!verifyAuthDigest(raw, user.AuthenticationProtocol, localUser.SecretKey, usp.AuthenticationParameters) {
		// Authentication digest doesn't match: the request claims to be
		// from this user but wasn't signed with their real passphrase.
		// Drop silently, same as an unrecognised user/community.
		return nil, nil
	}

	if decodeErr != nil {
		// The first decode attempt used blank (no-auth) security
		// parameters, which can't verify/decrypt an authenticated request —
		// retry with the real user's credentials now that we know them.
		retry := gosnmp.GoSNMP{Logger: discardLogger}
		retry.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 user.UserName,
			AuthenticationProtocol:   user.AuthenticationProtocol,
			PrivacyProtocol:          user.PrivacyProtocol,
			AuthenticationPassphrase: user.AuthenticationPassphrase,
			PrivacyPassphrase:        user.PrivacyPassphrase,
		}
		var err error
		req, err = retry.SnmpDecodePacket(raw)
		if err != nil {
			return nil, fmt.Errorf("decode authenticated snmpv3 packet: %w", err)
		}
	}

	resp, serveErr := a.serve(req)
	if serveErr != nil {
		return nil, nil //nolint:nilerr // unsupported/unknown PDU: drop silently, not a transport error
	}

	// Refresh the engine time to "now" for the response signature (it was
	// captured earlier, before request processing/serving).
	localUser.AuthoritativeEngineTime = a.engineTime()
	dummy := &gosnmp.SnmpPacket{MsgFlags: gosnmp.AuthPriv, SecurityParameters: &localUser}
	if err := localUser.InitPacket(dummy); err != nil {
		return nil, fmt.Errorf("init packet salt: %w", err)
	}
	resp.SecurityParameters = &localUser
	return resp.MarshalMsg()
}

func (a *agent) blankUSM() *gosnmp.UsmSecurityParameters {
	return &gosnmp.UsmSecurityParameters{
		AuthoritativeEngineID:    a.engineID,
		AuthoritativeEngineBoots: a.engineBoots,
		AuthoritativeEngineTime:  a.engineTime(),
	}
}

func (a *agent) findUser(name string) *gosnmp.UsmSecurityParameters {
	for i := range a.users {
		if a.users[i].UserName == name {
			return &a.users[i]
		}
	}
	return nil
}

// authDigestLen returns the number of leading digest bytes USM keeps after
// truncating the keyed hash, per the macVarbinds table in RFC 3414 §6.3.2 /
// RFC 7860 §4.2.2 (MD5/SHA-1 use 12; the SHA-2 family scales with hash size).
func authDigestLen(proto gosnmp.SnmpV3AuthProtocol) int {
	switch proto {
	case gosnmp.SHA224:
		return 16
	case gosnmp.SHA256:
		return 24
	case gosnmp.SHA384:
		return 32
	case gosnmp.SHA512:
		return 48
	default: // MD5, SHA
		return 12
	}
}

// verifyAuthDigest checks that raw — the exact bytes received on the wire —
// carries a valid USM authentication digest for the given localized key, per
// RFC 3414 §6.3.2 / RFC 7860 §4.2.2: a keyed hash computed over the whole
// message with the msgAuthenticationParameters field's content zeroed out,
// truncated, and compared against the digest the sender included.
//
// gosnmp.SnmpDecodePacket only decodes/decrypts a v3 packet — unlike its
// client-side request/response path, it never checks this digest — so
// without this, any client that merely knows a configured v3 username (no
// correct passphrase required) could impersonate that user.
//
// The digest's on-wire location is found by searching raw for the already-
// decoded digest bytes themselves, rather than re-parsing BER offsets. This
// is safe even in the astronomically unlikely case of a false match
// elsewhere in the packet: zeroing the wrong span only changes the bytes fed
// into our recomputation, so it can cause a legitimate request to be
// rejected (a false negative), never cause a forged request to be accepted.
func verifyAuthDigest(raw []byte, proto gosnmp.SnmpV3AuthProtocol, secretKey []byte, received string) bool {
	sentDigest := []byte(received)
	n := authDigestLen(proto)
	if len(sentDigest) != n {
		return false
	}
	idx := bytes.Index(raw, sentDigest)
	if idx < 0 {
		return false
	}
	zeroed := make([]byte, len(raw))
	copy(zeroed, raw)
	for i := range sentDigest {
		zeroed[idx+i] = 0
	}
	mac := hmac.New(proto.HashType().New, secretKey)
	mac.Write(zeroed)
	digest := mac.Sum(nil)
	if len(digest) < n {
		return false
	}
	return subtle.ConstantTimeCompare(digest[:n], sentDigest) == 1
}


func helloVariable() gosnmp.SnmpPDU {
	// usmStatsUnknownEngineIDs — http://oidref.com/1.3.6.1.6.3.15.1.1.4.0
	return gosnmp.SnmpPDU{Name: "1.3.6.1.6.3.15.1.1.4.0", Type: gosnmp.Counter32, Value: uint32(0)}
}

func endOfMibView(name string) gosnmp.SnmpPDU {
	return gosnmp.SnmpPDU{Name: name, Type: gosnmp.EndOfMibView, Value: nil}
}

func noSuchInstance(name string) gosnmp.SnmpPDU {
	return gosnmp.SnmpPDU{Name: name, Type: gosnmp.NoSuchInstance, Value: nil}
}

func nilPDU(name string) gosnmp.SnmpPDU {
	return gosnmp.SnmpPDU{Name: name, Type: gosnmp.Null, Value: nil}
}

func octetStringPDU(name, s string) gosnmp.SnmpPDU {
	return gosnmp.SnmpPDU{Name: name, Type: gosnmp.OctetString, Value: s}
}

// ---------------------------------------------------------------------------
// PDU dispatch (Get/GetNext/GetBulk/Set)
// ---------------------------------------------------------------------------

func (a *agent) serve(req *gosnmp.SnmpPacket) (*gosnmp.SnmpPacket, error) {
	switch req.PDUType {
	case gosnmp.GetRequest:
		return a.serveGet(req)
	case gosnmp.GetNextRequest:
		return a.serveGetNext(req)
	case gosnmp.GetBulkRequest:
		return a.serveGetBulk(req)
	case gosnmp.SetRequest:
		return a.serveSet(req)
	default:
		return nil, fmt.Errorf("unsupported pdu type %v", req.PDUType)
	}
}

func responseFor(req *gosnmp.SnmpPacket) *gosnmp.SnmpPacket {
	return &gosnmp.SnmpPacket{
		Version:            req.Version,
		MsgFlags:           req.MsgFlags,
		SecurityModel:      req.SecurityModel,
		SecurityParameters: req.SecurityParameters,
		ContextEngineID:    req.ContextEngineID,
		ContextName:        req.ContextName,
		Community:          req.Community,
		PDUType:            gosnmp.GetResponse,
		MsgID:              req.MsgID,
		RequestID:          req.RequestID,
		Variables:          []gosnmp.SnmpPDU{},
	}
}

func (a *agent) serveGet(req *gosnmp.SnmpPacket) (*gosnmp.SnmpPacket, error) {
	resp := responseFor(req)
	for idx, v := range req.Variables {
		item, _ := findOID(a.oids, v.Name)
		if item == nil {
			if resp.Error == gosnmp.NoError {
				resp.Error = gosnmp.NoSuchName
				resp.ErrorIndex = uint8(idx) //nolint:gosec // idx is bounded by request variable count
			}
			resp.Variables = append(resp.Variables, noSuchInstance(v.Name))
			continue
		}
		resp.Variables = append(resp.Variables, a.get(item))
	}
	return resp, nil
}

func (a *agent) serveGetNext(req *gosnmp.SnmpPacket) (*gosnmp.SnmpPacket, error) {
	resp := responseFor(req)
	if len(req.Variables) == 0 {
		return resp, nil
	}
	// GoSNMPServer's GetNext only ever looked at the last requested
	// variable; mockly's own client usage (and its existing test suite)
	// only ever requests GetNext for a single OID, so this preserves
	// existing, verified behavior rather than a theoretical multi-varbind
	// walk.
	query := req.Variables[len(req.Variables)-1].Name
	item, idx := findOID(a.oids, query)
	if item != nil {
		idx++
	}
	if idx >= len(a.oids) {
		resp.Variables = append(resp.Variables, endOfMibView(query))
		return resp, nil
	}
	limit := 1
	if req.MaxRepetitions != 0 {
		limit = int(req.MaxRepetitions)
	}
	if idx+limit > len(a.oids) {
		limit = len(a.oids) - idx
	}
	for i := idx; i < idx+limit; i++ {
		resp.Variables = append(resp.Variables, a.get(a.oids[i]))
	}
	if len(resp.Variables) == 0 {
		resp.Variables = append(resp.Variables, endOfMibView(query))
	}
	return resp, nil
}

func (a *agent) serveGetBulk(req *gosnmp.SnmpPacket) (*gosnmp.SnmpPacket, error) {
	resp := responseFor(req)
	n := len(req.Variables)
	nonRepeaters := int(req.NonRepeaters)
	if nonRepeaters > n {
		nonRepeaters = n
	}

	// Non-repeaters behave like a plain GetNext (lexicographic successor),
	// per RFC 3416 §4.2.3 — not an exact-match Get.
	for i := 0; i < nonRepeaters; i++ {
		query := req.Variables[i].Name
		item, idx := findOID(a.oids, query)
		if item != nil {
			idx++
		}
		if idx >= len(a.oids) {
			resp.Variables = append(resp.Variables, endOfMibView(query))
			continue
		}
		resp.Variables = append(resp.Variables, a.get(a.oids[idx]))
	}

	// Repeaters behave like repeated GetNext calls. MaxRepetitions is
	// client-controlled and can be up to ~2.1 billion; cap it at the OID
	// table size (no repeater can produce more than that many real values
	// before every repeater hits end-of-MIB) so a crafted packet can't hang
	// this agent's single-threaded request loop.
	maxReps := int(req.MaxRepetitions)
	if maxReps > len(a.oids) {
		maxReps = len(a.oids)
	}
	eomSent := make(map[string]bool)
	for rep := 0; rep < maxReps; rep++ {
		for k := nonRepeaters; k < n; k++ {
			query := req.Variables[k].Name
			item, idx := findOID(a.oids, query)
			if item != nil {
				idx++
			}
			next := idx + rep
			if next >= len(a.oids) {
				if !eomSent[query] {
					resp.Variables = append(resp.Variables, endOfMibView(query))
					eomSent[query] = true
				}
				continue
			}
			resp.Variables = append(resp.Variables, a.get(a.oids[next]))
		}
		if len(eomSent) == n-nonRepeaters {
			break // every repeater has hit end-of-MIB; further reps are no-ops
		}
	}
	return resp, nil
}

func (a *agent) serveSet(req *gosnmp.SnmpPacket) (*gosnmp.SnmpPacket, error) {
	resp := responseFor(req)
	for idx, v := range req.Variables {
		item, _ := findOID(a.oids, v.Name)
		switch {
		case item == nil:
			if resp.Error == gosnmp.NoError {
				resp.Error = gosnmp.NoSuchName
				resp.ErrorIndex = uint8(idx) //nolint:gosec // idx is bounded by request variable count
			}
			resp.Variables = append(resp.Variables, noSuchInstance(v.Name))
		case item.onSet == nil:
			if resp.Error == gosnmp.NoError {
				resp.Error = gosnmp.ReadOnly
				resp.ErrorIndex = uint8(idx) //nolint:gosec // idx is bounded by request variable count
			}
			resp.Variables = append(resp.Variables, nilPDU(v.Name))
		default:
			if err := safeSet(item, v.Value); err != nil {
				resp.Variables = append(resp.Variables, octetStringPDU(v.Name, fmt.Sprintf("ERROR: %v", err)))
			} else {
				resp.Variables = append(resp.Variables, v)
			}
		}
	}
	return resp, nil
}

// get evaluates item's OnGet callback, converting an error into an
// OctetString PDU carrying the error text rather than a packet-level SNMP
// error — matching mockly's existing (GoSNMPServer-era) fault-injection
// behavior of returning NoError with a visible "ERROR: ..." marker value.
func (a *agent) get(item *pduItem) gosnmp.SnmpPDU {
	val, err := safeGet(item)
	if err != nil {
		return octetStringPDU(item.oid, fmt.Sprintf("ERROR: %v", err))
	}
	return gosnmp.SnmpPDU{Name: item.oid, Type: item.typ, Value: val}
}

func safeGet(item *pduItem) (val interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return item.onGet()
}

func safeSet(item *pduItem, value interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return item.onSet(value)
}
