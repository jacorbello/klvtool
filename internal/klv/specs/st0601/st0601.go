// Package st0601 provides MISB ST 0601 UAS Datalink Local Set spec versions.
package st0601

// UASDatalinkUL is the 16-byte Universal Label for the UAS Datalink LS
// registered in MISB ST 0807. See ST 0601.19 §6.2.
var UASDatalinkUL = []byte{
	0x06, 0x0e, 0x2b, 0x34, 0x02, 0x0b, 0x01, 0x01,
	0x0e, 0x01, 0x03, 0x01, 0x01, 0x00, 0x00, 0x00,
}
