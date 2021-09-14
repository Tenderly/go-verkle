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
	"crypto/sha256"

	"github.com/ethereum/go-ethereum/common"
	"github.com/protolambda/go-kzg"
	"github.com/protolambda/go-kzg/bls"
)

func calcR(cs []*bls.G1Point, indices []int, ys []*bls.Fr, tc *KZGConfig) bls.Fr {
	digest := sha256.New()
	for _, c := range cs {
		h := sha256.Sum256(bls.ToCompressedG1(c))
		digest.Write(h[:])
	}
	for _, idx := range indices {
		tmp := bls.FrTo32(&tc.omegaIs[idx])
		digest.Write(tmp[:])
	}
	for _, y := range ys {
		tmp := bls.FrTo32(y)
		digest.Write(tmp[:])
	}

	var tmp bls.Fr
	hashToFr(&tmp, common.BytesToHash(digest.Sum(nil)))
	return tmp

}

func calcT(r *bls.Fr, d *bls.G1Point) bls.Fr {
	digest := sha256.New()

	tmpBytes := bls.FrTo32(r)
	digest.Write(tmpBytes[:])
	tmpBytes = sha256.Sum256(bls.ToCompressedG1(d))
	digest.Write(tmpBytes[:])

	var tmp bls.Fr
	hashToFr(&tmp, common.BytesToHash(digest.Sum(nil)))
	return tmp
}

func calcQ(e, d *bls.G1Point, y, w *bls.Fr) bls.Fr {
	digest := sha256.New()
	hE := sha256.Sum256(bls.ToCompressedG1(e))
	hD := sha256.Sum256(bls.ToCompressedG1(d))

	digest.Write(hE[:])
	digest.Write(hD[:])
	tmpBytes := bls.FrTo32(y)
	digest.Write(tmpBytes[:])
	tmpBytes = bls.FrTo32(w)
	digest.Write(tmpBytes[:])

	var tmp bls.Fr
	hashToFr(&tmp, common.BytesToHash(digest.Sum(nil)))
	return tmp
}

func ComputeKZGProof(tc *KZGConfig, poly []bls.Fr, z, y *bls.Fr) *bls.G1Point {
	oq := tc.outerQuotients(poly, z, y)
	return kzg.CommitToEvalPoly(tc.lg1, oq)
}

func MakeVerkleProofOneLeaf(root VerkleNode, key []byte) (d *bls.G1Point, y *bls.Fr, sigma *bls.G1Point) {
	root.ComputeCommitment()
	tc := GetKZGConfig()

	var fis [][]bls.Fr
	commitments, indices, yis, fis := root.GetCommitmentsAlongPath(key)

	// Construct g(x)
	r := calcR(commitments, indices, yis, tc)

	g := make([]bls.Fr, NodeWidth)
	var powR bls.Fr
	bls.CopyFr(&powR, &bls.ONE)
	for level, index := range indices {
		f := fis[level]
		quotients := GetKZGConfig().innerQuotients(f, index)
		var tmp bls.Fr
		for i := 0; i < NodeWidth; i++ {
			bls.MulModFr(&tmp, &powR, &quotients[i])
			bls.AddModFr(&g[i], &g[i], &tmp)
		}

		// rⁱ⁺¹ = r ⨯ rⁱ
		bls.MulModFr(&powR, &powR, &r)
	}
	d = GetKZGConfig().CommitToPoly(g, len(g))
	// Compute h(x)
	t := calcT(&r, d)

	h := make([]bls.Fr, NodeWidth)
	bls.CopyFr(&powR, &bls.ONE)
	for level, index := range indices {
		f := fis[level]
		var denom bls.Fr
		bls.SubModFr(&denom, &t, &tc.omegaIs[index])
		bls.DivModFr(&denom, &powR, &denom)

		for i := 0; i < NodeWidth; i++ {
			var tmp bls.Fr
			bls.MulModFr(&tmp, &denom, &f[i])
			bls.AddModFr(&h[i], &h[i], &tmp)
		}

		// rⁱ⁺¹ = r ⨯ rⁱ
		bls.MulModFr(&powR, &powR, &r)
	}

	// compute y and w by evaluating g and h at t. Since
	// t is outside the evaluation domain, each f_i(t)
	// is evaluated using the barycentric formula.
	y = new(bls.Fr)
	w := new(bls.Fr)
	for i := range g {
		var factor, tmp bls.Fr
		bls.SubModFr(&factor, &t, &tc.omegaIs[i])
		bls.DivModFr(&factor, &tc.omegaIs[i], &factor)

		bls.MulModFr(&tmp, &h[i], &factor)
		bls.AddModFr(y, y, &tmp)
		bls.MulModFr(&tmp, &g[i], &factor)
		bls.AddModFr(w, w, &tmp)
	}
	// Compute (t^width - 1)/width
	var tPowWidth bls.Fr
	bls.CopyFr(&tPowWidth, &t)
	for i := 0; i < NodeBitWidth; i++ {
		bls.MulModFr(&tPowWidth, &tPowWidth, &tPowWidth)
	}
	bls.SubModFr(&tPowWidth, &tPowWidth, &bls.ONE)
	bls.MulModFr(&tPowWidth, &tPowWidth, &tc.nodeWidthInversed)
	bls.MulModFr(w, w, &tPowWidth)
	bls.MulModFr(y, y, &tPowWidth)

	// compute π and ρ
	pi := ComputeKZGProof(tc, h, &t, y)
	rho := ComputeKZGProof(tc, g, &t, w)

	// Compute E
	e := kzg.CommitToEvalPoly(tc.lg1, h)

	// compute σ
	sigma = new(bls.G1Point)
	q := calcQ(e, d, y, w)
	bls.MulG1(sigma, rho, &q)
	bls.AddG1(sigma, sigma, pi)

	return
}

func GetCommitmentsForMultiproof(root VerkleNode, keys [][]byte) ([]*bls.G1Point, []int, []*bls.Fr, [][]bls.Fr) {
	var (
		fis         [][]bls.Fr
		commitments []*bls.G1Point
		indices     []int
		yis         []*bls.Fr
	)

	for _, key := range keys {
		cs, idxs, ys, fs := root.GetCommitmentsAlongPath(key)
		commitments = append(commitments, cs...)
		indices = append(indices, idxs...)
		yis = append(yis, ys...)
		fis = append(fis, fs...)
	}

	return commitments, indices, yis, fis
}

// Naive implementation, in which common keys are duplicated.
func MakeVerkleMultiProof(root VerkleNode, keys [][]byte) (d *bls.G1Point, y *bls.Fr, σ *bls.G1Point) {
	var powR bls.Fr

	tc := GetKZGConfig()
	root.ComputeCommitment()

	commitments, indices, yis, fis := GetCommitmentsForMultiproof(root, keys)

	// Construct g(x)
	r := calcR(commitments, indices, yis, tc)

	g := make([]bls.Fr, NodeWidth)
	bls.CopyFr(&powR, &bls.ONE)
	for level, index := range indices {
		f := fis[level]
		quotients := tc.innerQuotients(f, index)
		var tmp bls.Fr
		for i := 0; i < NodeWidth; i++ {
			bls.MulModFr(&tmp, &powR, &quotients[i])
			bls.AddModFr(&g[i], &g[i], &tmp)
		}

		// rⁱ⁺¹ = r ⨯ rⁱ
		bls.MulModFr(&powR, &powR, &r)
	}
	d = bls.LinCombG1(tc.lg1, g)

	// Compute h(x)
	t := calcT(&r, d)

	h := make([]bls.Fr, NodeWidth)
	bls.CopyFr(&powR, &bls.ONE)
	for level, index := range indices {
		f := fis[level]
		var denom bls.Fr
		bls.SubModFr(&denom, &t, &tc.omegaIs[index])
		bls.DivModFr(&denom, &powR, &denom)

		for i := 0; i < NodeWidth; i++ {
			var tmp bls.Fr
			bls.MulModFr(&tmp, &denom, &f[i])
			bls.AddModFr(&h[i], &h[i], &tmp)
		}

		// rⁱ⁺¹ = r ⨯ rⁱ
		bls.MulModFr(&powR, &powR, &r)
	}

	// compute y and w by evaluating g and h at t. Since
	// t is outside the evaluation domain, each f_i(t)
	// is evaluated using the barycentric formula.
	y = new(bls.Fr)
	w := new(bls.Fr)
	for i := range g {
		var factor, tmp bls.Fr
		bls.SubModFr(&factor, &t, &tc.omegaIs[i])
		bls.DivModFr(&factor, &tc.omegaIs[i], &factor)

		bls.MulModFr(&tmp, &h[i], &factor)
		bls.AddModFr(y, y, &tmp)
		bls.MulModFr(&tmp, &g[i], &factor)
		bls.AddModFr(w, w, &tmp)
	}
	// Compute (t^width - 1)/width
	var tPowWidth bls.Fr
	bls.CopyFr(&tPowWidth, &t)
	for i := 0; i < NodeBitWidth; i++ {
		bls.MulModFr(&tPowWidth, &tPowWidth, &tPowWidth)
	}
	bls.SubModFr(&tPowWidth, &tPowWidth, &bls.ONE)
	bls.MulModFr(&tPowWidth, &tPowWidth, &tc.nodeWidthInversed)
	bls.MulModFr(w, w, &tPowWidth)
	bls.MulModFr(y, y, &tPowWidth)

	// compute π and ρ
	pi := ComputeKZGProof(tc, h, &t, y)
	rho := ComputeKZGProof(tc, g, &t, w)

	// Compute E
	e := kzg.CommitToEvalPoly(tc.lg1, h)

	// compute σ
	σ = new(bls.G1Point)
	q := calcQ(e, d, y, w)
	bls.MulG1(σ, rho, &q)
	bls.AddG1(σ, σ, pi)

	return
}

func VerifyVerkleProof(ks *kzg.KZGSettings, d, sigma *bls.G1Point, y *bls.Fr, commitments []*bls.G1Point, indices []int, yis []*bls.Fr, tc *KZGConfig) bool {
	zis := make([]*bls.Fr, len(indices))
	for i, index := range indices {
		zis[i] = &tc.omegaIs[index]
	}
	r := calcR(commitments, indices, yis, tc)
	t := calcT(&r, d)

	// Evaluate w = g₂(t) and E
	var powR bls.Fr
	var e bls.G1Point
	bls.CopyFr(&powR, &bls.ONE)
	var g2t bls.Fr
	for i, index := range indices {
		var tMinusZi, rDivZi, tmp bls.Fr
		bls.SubModFr(&tMinusZi, &t, &tc.omegaIs[index])
		bls.DivModFr(&rDivZi, &powR, &tMinusZi)

		// g₂(t)
		bls.MulModFr(&tmp, &rDivZi, yis[i])
		bls.AddModFr(&g2t, &g2t, &tmp)

		// E
		var eTmp bls.G1Point
		bls.MulG1(&eTmp, commitments[i], &rDivZi)
		bls.AddG1(&e, &e, &eTmp)

		// rⁱ⁺¹ = r ⨯ rⁱ
		bls.MulModFr(&powR, &powR, &r)
	}

	// w = y - g₂(t)
	var w bls.Fr
	bls.SubModFr(&w, y, &g2t)

	// Calculate q
	q := calcQ(&e, d, y, &w)

	// final=E+qD
	var final bls.G1Point
	bls.MulG1(&final, d, &q)
	bls.AddG1(&final, &final, &e)

	// finalAt=y+w*q
	var finalAt bls.Fr
	bls.MulModFr(&finalAt, &q, &w)
	bls.AddModFr(&finalAt, &finalAt, y)

	return ks.CheckProofSingle(&final, sigma, &t, &finalAt)
}
