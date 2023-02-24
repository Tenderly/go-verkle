// This is free and unencumbered software released into the public domain.
//
// Anyone is free to copy, modify, publish, use, compile, sell, or
// distribute this software, either in source code form or as a compiled
// binary, for any purpose, commercial or non-commercial, and by any
// means.
//
// In jurisdictions that recognize copyright laws, the author or authors
// of this software dedicate any and all copyright interest in the
// software to the public domain. We make this dedication for the benefit
// of the public at large and to the detriment of our heirs and
// successors. We intend this dedication to be an overt act of
// relinquishment in perpetuity of all present and future rights to this
// software under copyright law.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
// IN NO EVENT SHALL THE AUTHORS BE LIABLE FOR ANY CLAIM, DAMAGES OR
// OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
// ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
// OTHER DEALINGS IN THE SOFTWARE.
//
// For more information, please refer to <https://unlicense.org>

package verkle

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"unsafe"

	ipa "github.com/crate-crypto/go-ipa"
	"github.com/crate-crypto/go-ipa/common"
)

const IPA_PROOF_DEPTH = 8

type IPAProof struct {
	CL              [IPA_PROOF_DEPTH][32]byte `json:"cl"`
	CR              [IPA_PROOF_DEPTH][32]byte `json:"cr"`
	FinalEvaluation [32]byte                  `json:"finalEvaluation"`
}

type ipaproofMarshaller struct {
	CL              [IPA_PROOF_DEPTH]string `json:"cl"`
	CR              [IPA_PROOF_DEPTH]string `json:"cr"`
	FinalEvaluation string                  `json:"finalEvaluation"`
}

func (ipp *IPAProof) MarshalJSON() ([]byte, error) {
	return json.Marshal(&ipaproofMarshaller{
		CL: [IPA_PROOF_DEPTH]string{
			hex.EncodeToString(ipp.CL[0][:]),
			hex.EncodeToString(ipp.CL[1][:]),
			hex.EncodeToString(ipp.CL[2][:]),
			hex.EncodeToString(ipp.CL[3][:]),
			hex.EncodeToString(ipp.CL[4][:]),
			hex.EncodeToString(ipp.CL[5][:]),
			hex.EncodeToString(ipp.CL[6][:]),
			hex.EncodeToString(ipp.CL[7][:]),
		},
		CR: [IPA_PROOF_DEPTH]string{
			hex.EncodeToString(ipp.CR[0][:]),
			hex.EncodeToString(ipp.CR[1][:]),
			hex.EncodeToString(ipp.CR[2][:]),
			hex.EncodeToString(ipp.CR[3][:]),
			hex.EncodeToString(ipp.CR[4][:]),
			hex.EncodeToString(ipp.CR[5][:]),
			hex.EncodeToString(ipp.CR[6][:]),
			hex.EncodeToString(ipp.CR[7][:]),
		},
		FinalEvaluation: hex.EncodeToString(ipp.FinalEvaluation[:]),
	})
}

func (ipp *IPAProof) UnmarshalJSON(data []byte) error {
	aux := &ipaproofMarshaller{}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if len(aux.FinalEvaluation) != 64 {
		return fmt.Errorf("invalid hex string for final evaluation: %s", aux.FinalEvaluation)
	}

	currentValueBytes, err := hex.DecodeString(aux.FinalEvaluation)
	if err != nil {
		return fmt.Errorf("error decoding hex string for current value: %v", err)
	}
	copy(ipp.FinalEvaluation[:], currentValueBytes)

	for i := range ipp.CL {
		if len(aux.CL[i]) != 64 {
			return fmt.Errorf("invalid hex string for CL[%d]: %s", i, aux.CL[i])
		}
		val, err := hex.DecodeString(aux.CL[i])
		if err != nil {
			return fmt.Errorf("error decoding hex string for CL[%d]: %s", i, aux.CL[i])
		}
		copy(ipp.CL[i][:], val[:])
		if len(aux.CR[i]) != 64 {
			return fmt.Errorf("invalid hex string for CR[%d]: %s", i, aux.CR[i])
		}
		val, err = hex.DecodeString(aux.CR[i])
		if err != nil {
			return fmt.Errorf("error decoding hex string for CR[%d]: %s", i, aux.CR[i])
		}
		copy(ipp.CR[i][:], val[:])
	}
	copy(ipp.FinalEvaluation[:], currentValueBytes)

	return nil
}

type VerkleProof struct {
	OtherStems            [][31]byte `json:"otherStems"`
	DepthExtensionPresent []byte     `json:"depthExtensionPresent"`
	CommitmentsByPath     [][32]byte `json:"commitmentsByPath"`
	D                     [32]byte   `json:"d"`
	IPAProof              *IPAProof  `json:"ipa_proof"`
}

type verkleProofMarshaller struct {
	OtherStems            []string  `json:"otherStems"`
	DepthExtensionPresent string    `json:"depthExtensionPresent"`
	CommitmentsByPath     []string  `json:"commitmentsByPath"`
	D                     string    `json:"d"`
	IPAProof              *IPAProof `json:"ipa_proof"`
}

func (vp *VerkleProof) MarshalJSON() ([]byte, error) {
	aux := &verkleProofMarshaller{
		OtherStems:            make([]string, len(vp.OtherStems)),
		DepthExtensionPresent: hex.EncodeToString(vp.DepthExtensionPresent[:]),
		CommitmentsByPath:     make([]string, len(vp.CommitmentsByPath)),
		D:                     hex.EncodeToString(vp.D[:]),
		IPAProof:              vp.IPAProof,
	}

	for i, s := range vp.OtherStems {
		aux.OtherStems[i] = hex.EncodeToString(s[:])
	}
	for i, c := range vp.CommitmentsByPath {
		aux.CommitmentsByPath[i] = hex.EncodeToString(c[:])
	}
	return json.Marshal(aux)
}

func (vp *VerkleProof) UnmarshalJSON(data []byte) error {
	var aux verkleProofMarshaller
	err := json.Unmarshal(data, &aux)
	if err != nil {
		return err
	}

	vp.DepthExtensionPresent, err = hex.DecodeString(aux.DepthExtensionPresent)
	if err != nil {
		return fmt.Errorf("error decoding hex string for depth and extention present: %v", err)
	}

	vp.CommitmentsByPath = make([][32]byte, len(aux.CommitmentsByPath))
	for i, c := range aux.CommitmentsByPath {
		val, err := hex.DecodeString(c)
		if err != nil {
			return fmt.Errorf("error decoding hex string for commitment #%d: %w", i, err)
		}
		copy(vp.CommitmentsByPath[i][:], val)
	}

	currentValueBytes, err := hex.DecodeString(aux.D)
	if err != nil {
		return fmt.Errorf("error decoding hex string for D: %w", err)
	}
	copy(vp.D[:], currentValueBytes)

	vp.OtherStems = make([][31]byte, len(aux.OtherStems))
	for i, c := range aux.OtherStems {
		val, err := hex.DecodeString(c)
		if err != nil {
			return fmt.Errorf("error decoding hex string for other stem #%d: %w", i, err)
		}
		copy(vp.OtherStems[i][:], val)
	}

	vp.IPAProof = aux.IPAProof
	return nil
}

type Proof struct {
	Multipoint *ipa.MultiProof // multipoint argument
	ExtStatus  []byte          // the extension status of each stem
	Cs         []*Point        // commitments, sorted by their path in the tree
	PoaStems   [][]byte        // stems proving another stem is absent
	Keys       [][]byte
	Values     [][]byte
}

type SuffixStateDiff struct {
	Suffix       byte      `json:"suffix"`
	CurrentValue *[32]byte `json:"currentValue"`
}

type suffixStateDiffMarshaller struct {
	Suffix       byte   `json:"suffix"`
	CurrentValue string `json:"currentValue"`
}

func (ssd SuffixStateDiff) MarshalJSON() ([]byte, error) {
	return json.Marshal(&suffixStateDiffMarshaller{
		Suffix:       ssd.Suffix,
		CurrentValue: hex.EncodeToString(ssd.CurrentValue[:]),
	})
}

func (ssd *SuffixStateDiff) UnmarshalJSON(data []byte) error {
	aux := &suffixStateDiffMarshaller{
		CurrentValue: "",
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if len(aux.CurrentValue) != 64 {
		return fmt.Errorf("invalid hex string for current value: %s", aux.CurrentValue)
	}

	currentValueBytes, err := hex.DecodeString(aux.CurrentValue)
	if err != nil {
		return fmt.Errorf("error decoding hex string for current value: %v", err)
	}

	*ssd = SuffixStateDiff{
		Suffix:       aux.Suffix,
		CurrentValue: &[32]byte{},
	}

	copy(ssd.CurrentValue[:], currentValueBytes)

	return nil
}

type SuffixStateDiffs []SuffixStateDiff

type StemStateDiff struct {
	Stem        [31]byte         `json:"stem"`
	SuffixDiffs SuffixStateDiffs `json:"suffixDiffs"`
}

type StateDiff []StemStateDiff

func GetCommitmentsForMultiproof(root VerkleNode, keys [][]byte) (*ProofElements, []byte, [][]byte) {
	sort.Sort(keylist(keys))
	return root.GetProofItems(keylist(keys))
}

func MakeVerkleMultiProof(root VerkleNode, keys [][]byte, keyvals map[string][]byte) (*Proof, []*Point, []byte, []*Fr, error) {
	// go-ipa won't accept no key as an input, catch this corner case
	// and return an empty result.
	if len(keys) == 0 {
		return nil, nil, nil, nil, errors.New("no key provided for proof")
	}

	tr := common.NewTranscript("vt")
	root.Commit()

	pe, es, poas := GetCommitmentsForMultiproof(root, keys)

	var vals [][]byte
	for _, k := range keys {
		// TODO at the moment, do not include the post-data
		//val, _ := root.Get(k, nil)
		//vals = append(vals, val)
		vals = append(vals, keyvals[string(k)])
	}

	cfg := GetConfig()
	mpArg := ipa.CreateMultiProof(tr, cfg.conf, pe.Cis, pe.Fis, pe.Zis)

	// It's wheel-reinvention time again 🎉: reimplement a basic
	// feature that should be part of the stdlib.
	// "But golang is a high-productivity language!!!" 🤪
	// len()-1, because the root is already present in the
	// parent block, so we don't keep it in the proof.
	paths := make([]string, 0, len(pe.ByPath)-1)
	for path := range pe.ByPath {
		if len(path) > 0 {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	cis := make([]*Point, len(pe.ByPath)-1)
	for i, path := range paths {
		cis[i] = pe.ByPath[path]
	}
	proof := &Proof{
		Multipoint: mpArg,
		Cs:         cis,
		ExtStatus:  es,
		PoaStems:   poas,
		Keys:       keys,
		Values:     vals,
	}
	return proof, pe.Cis, pe.Zis, pe.Yis, nil
}

func VerifyVerkleProof(proof *Proof, Cs []*Point, indices []uint8, ys []*Fr, tc *Config) bool {
	tr := common.NewTranscript("vt")
	return ipa.CheckMultiProof(tr, tc.conf, proof.Multipoint, Cs, ys, indices)
}

// SerializeProof serializes the proof in the rust-verkle format:
// * len(Proof of absence stem) || Proof of absence stems
// * len(depths) || serialize(depth || ext statusi)
// * len(commitments) || serialize(commitment)
// * Multipoint proof
// it also returns the serialized keys and values
func SerializeProof(proof *Proof) (*VerkleProof, StateDiff, error) {
	otherstems := make([][31]byte, len(proof.PoaStems))
	for i, stem := range proof.PoaStems {
		copy(otherstems[i][:], stem[:])
	}

	cbp := make([][32]byte, len(proof.Cs))
	for i, C := range proof.Cs {
		serialized := C.Bytes()
		copy(cbp[i][:], serialized[:])
	}

	var cls, crs [IPA_PROOF_DEPTH][32]byte
	for i := 0; i < IPA_PROOF_DEPTH; i++ {

		l := proof.Multipoint.IPA.L[i].Bytes()
		copy(cls[i][:], l[:])
		r := proof.Multipoint.IPA.R[i].Bytes()
		copy(crs[i][:], r[:])
	}

	var stemdiff *StemStateDiff
	var statediff StateDiff
	for i, key := range proof.Keys {
		if stemdiff == nil || !bytes.Equal(stemdiff.Stem[:], key[:31]) {
			statediff = append(statediff, StemStateDiff{})
			stemdiff = &statediff[len(statediff)-1]
			copy(stemdiff.Stem[:], key[:31])
		}
		var valueLen = len(proof.Values[i])
		switch valueLen {
		case 0:
			stemdiff.SuffixDiffs = append(stemdiff.SuffixDiffs, SuffixStateDiff{
				Suffix: key[31],
			})
		case 32:
			stemdiff.SuffixDiffs = append(stemdiff.SuffixDiffs, SuffixStateDiff{
				Suffix:       key[31],
				CurrentValue: (*[32]byte)(proof.Values[i]),
			})
		default:
			var aligned [32]byte
			copy(aligned[:valueLen], proof.Values[i])
			stemdiff.SuffixDiffs = append(stemdiff.SuffixDiffs, SuffixStateDiff{
				Suffix:       key[31],
				CurrentValue: (*[32]byte)(unsafe.Pointer(&aligned[0])),
			})
		}
	}
	return &VerkleProof{
		OtherStems:            otherstems,
		DepthExtensionPresent: proof.ExtStatus,
		CommitmentsByPath:     cbp,
		D:                     proof.Multipoint.D.Bytes(),
		IPAProof: &IPAProof{
			CL:              cls,
			CR:              crs,
			FinalEvaluation: proof.Multipoint.IPA.A_scalar.Bytes(),
		},
	}, statediff, nil
}

// DeserializeProof deserializes the proof found in blocks, into a format that
// can be used to rebuild a stateless version of the tree.
func DeserializeProof(vp *VerkleProof, statediff StateDiff) (*Proof, error) {
	var (
		poaStems, keys, values [][]byte
		extStatus              []byte
		commitments            []*Point
		multipoint             ipa.MultiProof
	)

	poaStems = make([][]byte, len(vp.OtherStems))
	for i, poaStem := range vp.OtherStems {
		poaStems[i] = poaStem[:]
	}

	extStatus = vp.DepthExtensionPresent

	commitments = make([]*Point, len(vp.CommitmentsByPath))
	for i, commitmentBytes := range vp.CommitmentsByPath {
		var commitment Point
		if err := commitment.SetBytesTrusted(commitmentBytes[:]); err != nil {
			return nil, err
		}
		commitments[i] = &commitment
	}

	multipoint.D.SetBytes(vp.D[:])
	multipoint.IPA.A_scalar.SetBytes(vp.IPAProof.FinalEvaluation[:])
	multipoint.IPA.L = make([]Point, IPA_PROOF_DEPTH)
	for i, b := range vp.IPAProof.CL {
		multipoint.IPA.L[i].SetBytes(b[:])
	}
	multipoint.IPA.R = make([]Point, IPA_PROOF_DEPTH)
	for i, b := range vp.IPAProof.CR {
		multipoint.IPA.R[i].SetBytes(b[:])
	}

	// turn statediff into keys and values
	for _, stemdiff := range statediff {
		for _, suffixdiff := range stemdiff.SuffixDiffs {
			var k [32]byte
			copy(k[:31], stemdiff.Stem[:])
			k[31] = suffixdiff.Suffix
			keys = append(keys, k[:])
			if suffixdiff.CurrentValue != nil {
				values = append(values, suffixdiff.CurrentValue[:])
			} else {
				values = append(values, nil)
			}
		}
	}

	proof := Proof{
		&multipoint,
		extStatus,
		commitments,
		poaStems,
		keys,
		values,
	}
	return &proof, nil
}

type stemInfo struct {
	depth          byte
	stemType       byte
	has_c1, has_c2 bool
	values         map[byte][]byte
	stem           []byte
}

// TreeFromProof builds a stateless tree from the proof
func TreeFromProof(proof *Proof, rootC *Point) (VerkleNode, error) {
	stems := make([][]byte, 0, len(proof.Keys))
	for _, k := range proof.Keys {
		if len(stems) == 0 || !bytes.Equal(stems[len(stems)-1], k[:31]) {
			stems = append(stems, k[:31])
		}
	}
	stemIndex := 0

	var (
		info  = map[string]stemInfo{}
		paths [][]byte
		err   error
		poas  = proof.PoaStems
	)

	// assign one or more stem to each stem info
	for _, es := range proof.ExtStatus {
		depth := es >> 3
		path := stems[stemIndex][:depth]
		si := stemInfo{
			depth:    depth,
			stemType: es & 3,
		}
		switch si.stemType {
		case extStatusAbsentEmpty:
		case extStatusAbsentOther:
			si.stem = poas[0]
			poas = poas[1:]
		default:
			// the first stem could be missing (e.g. the second stem in the
			// group is the one that is present. Compare each key to the first
			// stem, along the length of the path only.
			stemPath := stems[stemIndex][:len(path)]
			si.values = map[byte][]byte{}
			for i, k := range proof.Keys {
				if bytes.Equal(k[:len(path)], stemPath) && proof.Values[i] != nil {
					si.values[k[31]] = proof.Values[i]
					si.has_c1 = si.has_c1 || (k[31] < 128)
					si.has_c2 = si.has_c2 || (k[31] >= 128)
					// This key has values, its stem is the one that
					// is present.
					si.stem = k[:31]
				}
			}
		}
		info[string(path)] = si
		paths = append(paths, path)

		// Skip over all the stems that share the same path
		// to the extension tree. This happens e.g. if two
		// stems have the same path, but one is a proof of
		// absence and the other one is present.
		stemIndex++
		for ; stemIndex < len(stems); stemIndex++ {
			if !bytes.Equal(stems[stemIndex][:depth], path) {
				break
			}
		}
	}

	root := NewStatelessWithCommitment(rootC)
	comms := proof.Cs
	for _, p := range paths {
		comms, err = root.insertStem(p, info[string(p)], comms)
		if err != nil {
			return nil, err
		}
	}

	for i, k := range proof.Keys {
		if len(proof.Values[i]) == 0 {
			// Skip the nil keys, they are here to prove
			// an absence.
			continue
		}

		err = root.insertValue(k, proof.Values[i])
		if err != nil {
			return nil, err
		}
	}

	return root, nil
}
