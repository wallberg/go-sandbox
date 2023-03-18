package taocp

import (
	"fmt"
	"log"
	"math"
	"strings"
)

const (
	MaxInt = int(^uint(0) >> 1)
	rt     = MaxInt - 1 // RT - real truth
	nt     = MaxInt - 3 // NT - near truth
	pt     = MaxInt - 5 // PT - proto truth
)

// SatAlgorithmLOptions provides optional features
type SatAlgorithmLOptions struct {
	// Optional Compensation Resolvants (Exercise 139) - default false
	CompensationResolvants bool

	// Optional Big Clauses (Exercise 143) - default true
	SuppressBigClauses bool
}

// SatAlgorithmL implements Algorithm L (7.2.2.2), satisfiability by DPLL with lookahead.
// The task is to determine if the clause set is satisfiable, and if it is return
// one satisfying assignment of the clauses.
//
// Arguments:
// n       -- number of strictly distinct literals
// clauses -- list of clauses to satisfy
// stats   -- SAT processing statistics
// options -- runtime options
func SatAlgorithmL(n int, clauses SatClauses,
	stats *SatStats, options *SatOptions, optionsL *SatAlgorithmLOptions) (sat bool, solution []int) {

	// @note Global variables - some of the arrays are indexed by stack depth level (d) and some by fixed variables (F)
	var (
		// original value of n, before conversion to 3SAT
		nOrig int

		// d - depth of the implicit search tree
		d int

		// N - number of free variables in VAR (F + N = n)
		N int

		// F - number of fixed variables (F + N = n)
		F int

		// VAR - list of free variables; permutation of {1,...,n} (VAR[k] = x iff INX[x] = k)
		VAR []int

		// INX - index partner of VAR (free list; variable indexed X -> location in VAR)
		INX []int

		// X - variable of L promoted to real truth
		X int

		// BIMP - binary clauses; instead of the buddy system, we are using built-in slices
		BIMP [][]int

		// BSIZE - number of clauses for each l in BIMP
		BSIZE []int

		// TIMP - ternary clauses
		TIMP []int

		// TSIZE - number of clauses for each l in TIMP
		TSIZE []int

		// LINK - circular list of the three literals in each clause in TIMP
		LINK []int

		// KINX - sequential list of clauses c for each literal l
		// (Exercise 143 - "big" clauses of k > 2)
		KINX [][]int

		// KSIZE - current number of active clauses for each literal l
		// (Exercise 143 - "big" clauses of k > 2)
		KSIZE []int

		// BACKKSIZE - store previous versions of KSIZE in the stack
		// (Exercise 143 - "big" clauses of k > 2)
		BACKKSIZE []int

		// backksize - previous version of KSIZE popped from BACKKISZE
		backksize int

		// CINX - sequential list of literals l for each clause c
		// (Exercise 143 - "big" clauses of k > 2)
		CINX [][]int

		// CSIZE - current number of active literals l for each clause c
		// (Exercise 143 - "big" clauses of k > 2)
		CSIZE []int

		// CTHETA - Computed 𝜃 threshold for swapping free literals in CINX (L7)
		// (Exercise 143 - "big" clauses of k > 2)
		CTHETA []int

		// uvStack - temporary stack of (u, v) values, moving to BIMP table
		// (Exercise 143 - "big" clauses of k > 2)
		uvStack [][2]int

		// index into TIMP
		p, pp, ppp int

		// U - number of distinct variables in unit clauses (at the current depth)
		U int

		// FORCE - stack of U unit variables which have a forced value at the current depth
		FORCE []int

		// ISTAMP - stamp to make downdating BIMP tables easier
		ISTAMP int

		// IST - private stamp for literal l, with ISTAMP (variable indexed)
		IST []int

		// ISTACK - stack of previous values of (l, BSIZE[l])
		ISTACK [][2]int

		// I - size of ISTACK
		I int

		// BSTAMP - stamp to make downdating BIMP tables easier
		// (Compensation Resolvent, Exercise 139)
		BSTAMP int

		// BST - private stamp for literal l, with BSTAMP (variable indexed)
		// (Compensation Resolvent, Exercise 139)
		BST []int

		// BRANCH - decision making at depth d; {-1: no decision yet, 0: trying l, 1: trying ^l)} (depth indexed)
		BRANCH []int

		// DEC - decision on l at each branch (depth indexed)
		DEC []int

		// BACKI - store previous versions of I in the stack (depth indexed)
		BACKI []int

		// BACKF - store previous versions of F in the stack (depth indexed)
		BACKF []int

		// BACKL - added for showProgress(), Exercise 142. Appears to be identical to BACKF (depth indexed)
		BACKL []int

		// T - truth context
		T int

		// L - nearly true literal l
		L int

		// VAL - track if variable x is fixed in context T (variable indexed)
		VAL []int

		// R - record the names of literals that have received values (variable indexed)
		R []int

		// E - current stack size of R; 0 <= E <= n
		E int

		// G - number of really true literals in R (starting from 0), and nearly true for G <= k < E
		G int

		// CONFLICT - algorithm L step to goto in case of CONFLICT
		CONFLICT int

		// literal l
		l int

		// variable x
		x int

		// index
		k, j int

		// is debugging enabled?
		debug bool

		// is progress tracking enabled?
		progress bool

		// are we using "big" clauses?
		bigClauses bool
	)

	// // assertKinxIntegrity
	// assertKinxIntegrity := func() {
	// 	if !bigClauses {
	// 		return
	// 	}

	// 	// KINX
	// 	for l := 2; l <= 2*n+1; l++ {
	// 		if KSIZE[l] < 0 || KSIZE[l] > len(KINX[l]) {
	// 			log.Panicf("assertion failed: KSIZE[%d]=%d", l, KSIZE[l])
	// 		}
	// 	}
	// }

	// // assertTimpIntegrity
	// assertTimpIntegrity := func() {
	// 	if bigClauses {
	// 		return
	// 	}

	// 	for l := 2; l <= 2*n+1; l++ {
	// 		boundary := 0

	// 		// Set boundary to value of the next l' with clauses, ie TIMP[lp] > 0
	// 		for lp := l + 1; lp < 2*n+1; lp++ {
	// 			if TIMP[lp] > 0 {
	// 				boundary = TIMP[lp]
	// 				break
	// 			}
	// 		}
	// 		if boundary == 0 {
	// 			boundary = len(TIMP)
	// 		}
	// 		if TIMP[l]+2*TSIZE[l] > boundary {
	// 			log.Panicf("l=%d, boundary=%d, TSIZE[l]=%d", l, boundary, TSIZE[l])
	// 		}
	// 	}
	// }

	// truth returns a string description of truth values
	truth := func(t int) string {
		switch t {
		case rt + 1:
			return "RF"
		case rt:
			return "RT"
		case nt + 1:
			return "NF"
		case nt:
			return "NT"
		case pt + 1:
			return "PF"
		case pt:
			return "PT"
		default:
			return fmt.Sprintf("%d", t)
		}
	}

	// showProgress from Exercise 142
	// Move codes:
	// 0 - trying 1, haven't tried 0
	// 1 - trying 0, haven't tried 1
	// 2 - trying 1 after 0 failed
	// 3 - trying 0 after 1 failed
	// 4 - forced value is 1 (by BIMP reduction)
	// 5 - forced value is 0 (by BIMP reduction)
	// 6 - forced value is 1 (by input unit clause or Algorithm X)
	// 7 - forced value is 0 (by input unit clause or Algorithm X)

	showProgress := func() {

		var b strings.Builder
		b.WriteString(fmt.Sprintf("Progress: n=%d, d=%d, F=%d, E=%d, G=%d : ", n, d, F, E, G))

		r := 0
		k := 0

		for k < d {
			// Forced values (6, 7)
			for r < BACKF[k] {
				b.WriteString(fmt.Sprintf("%d ", 6+(R[r]&1)))
				r += 1
			}

			if BRANCH[k] < 0 {
				// No decision yet
				b.WriteString("| ")
			} else {
				// Trying values (0, 1, 2, 3)
				b.WriteString(fmt.Sprintf("%d ", (2*BRANCH[k])+R[r]&1))
				r += 1
			}

			// Forced values (4, 5)
			for r < BACKL[k+1] {
				b.WriteString(fmt.Sprintf("%d ", 4+(R[r]&1)))
				r += 1
			}

			k += 1
		}

		if debug {

			// misc variables and the R stack
			b.WriteString("\n")
			b.WriteString("            ")
			for k := 0; k < E; k++ {
				if k > 0 {
					b.WriteString(", ")
				}
				l := R[k]
				x := l >> 1
				b.WriteString(fmt.Sprintf("{%d}=%s", l, truth(VAL[x])))
			}
			b.WriteString("\n")

			// Statistics
			b.WriteString(fmt.Sprintf("            Nodes=%d, Levels=%v\n", stats.Nodes, stats.Levels))
		}

		b.WriteString("\n")
		log.Print(b.String())
	}

	// dump
	// @note dump()
	dump := func() {

		var b strings.Builder
		b.WriteString("\n")

		b.WriteString(fmt.Sprintf("n=%d, d=%d, F=%d\n", n, d, F))
		b.WriteString("\n")

		// FORCE
		b.WriteString("FORCE\n")
		b.WriteString(fmt.Sprintf("U=%d:", U))
		for i := 0; i < U; i++ {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(fmt.Sprintf(" {%d}", FORCE[i]))
		}
		b.WriteString("\n\n")

		// VAR
		b.WriteString("VAR\n")
		b.WriteString(fmt.Sprintf("N=%d: ", N))
		for k := 0; k < n; k++ {
			if k == N {
				b.WriteString(" | ")
			} else if k > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("{%d}", VAR[k]))
		}
		b.WriteString("\n\n")

		// R
		b.WriteString("R\n")
		b.WriteString(fmt.Sprintf("E=%d, G=%d: ", E, G))
		for k := 0; k < E; k++ {
			if k > 0 {
				b.WriteString(", ")
			}
			l := R[k]
			x := l >> 1
			b.WriteString(fmt.Sprintf("{%d}=%s", l, truth(VAL[x])))
		}
		b.WriteString("\n\n")

		// VAL
		b.WriteString("VAL\n")
		for x := 1; x <= n; x++ {
			if x > 1 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("{%d}=%s", x, truth(VAL[x])))
		}
		b.WriteString("\n\n")

		// BIMP
		b.WriteString("BIMP\n")
		for l := 2; l <= 2*n+1; l++ {
			b.WriteString(fmt.Sprintf("%d:", l))
			for i := 0; i < BSIZE[l]; i++ {
				if i > 0 {
					b.WriteString(",")
				}
				b.WriteString(fmt.Sprintf(" %d", BIMP[l][i]))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")

		if bigClauses {
			active := make([]bool, len(CINX)) // active clauses

			// KINX
			b.WriteString("KINX\n")
			for l := 2; l <= 2*n+1; l++ {
				var x string
				if INX[l>>1] < N {
					x = " "
				} else {
					x = "f"
					for k := 0; k < E; k++ {
						if R[k] == l {
							x = "t"
							break
						}
					}
				}
				b.WriteString(fmt.Sprintf("l=%s%d: ", x, l))

				for i, c := range KINX[l] {
					if x != "f" && i < KSIZE[l] {
						active[c] = true
					}
					if i == KSIZE[l] {
						b.WriteString(" | ")
					} else if i > 0 {
						b.WriteString(", ")
					}
					b.WriteString(fmt.Sprintf("%d", c))
				}
				b.WriteString("\n")
			}
			b.WriteString("\n")

			// CINX
			b.WriteString("CINX\n")
			for c := range CINX {
				a := "  a"
				if active[c] {
					a = "n/a"
				}
				b.WriteString(fmt.Sprintf("c=%d: size=%d: %s:", c, CSIZE[c], a))

				for i, l := range CINX[c] {
					if i > 0 {
						b.WriteString(",")
					}
					b.WriteString(" ")
					var x string
					if INX[l>>1] < N {
						x = " "
					} else {
						// x = "f"
						// for k := 0; k < E; k++ {
						// 	if R[k] == l {
						// 		x = "t"
						// 		break
						// 	}
						// }
						x = "x"
					}
					b.WriteString(fmt.Sprintf("%s%d", x, l))
				}
				b.WriteString("\n")
			}
			b.WriteString("\n")

		} else {
			// TIMP
			b.WriteString("TIMP\n")
			for l := 2; l <= 2*n+1; l++ {

				var boundary int
				if l < 2*n+1 {
					boundary = TIMP[l+1]
				} else {
					boundary = len(TIMP)
				}

				b.WriteString(fmt.Sprintf("l=%d: ", l))
				i := 0
				p := TIMP[l]
				for p < boundary {

					if i == TSIZE[l] {
						b.WriteString(" | ")
					} else if i > 0 {
						b.WriteString(", ")
					}

					// if p == 262 {
					// 	log.Panicf("l=%d, i=%d, TIMP[l]=%d, 2i=%d, p=%d\n", l, i, TIMP[l], 2*i, p)
					// }
					b.WriteString(fmt.Sprintf("{%d,%d}", TIMP[p], TIMP[p+1]))
					// pp = LINK[p]
					// b.WriteString(fmt.Sprintf("->{%d,%d}", TIMP[pp], TIMP[pp+1]))
					// ppp = LINK[pp]
					// b.WriteString(fmt.Sprintf("->{%d,%d}", TIMP[ppp], TIMP[ppp+1]))

					i++
					p += 2
				}
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
		log.Print(b.String())
	}

	// @note initialize()
	initialize := func() {

		if optionsL == nil {
			optionsL = &SatAlgorithmLOptions{}
		}

		if stats != nil {
			stats.Theta = stats.Delta
			stats.MaxLevel = -1
			if stats.Levels == nil {
				stats.Levels = make([]int, n+1)
			} else {
				for len(stats.Levels) < n {
					stats.Levels = append(stats.Levels, 0)
				}
			}
			debug = stats.Debug
			progress = stats.Progress
		}
	}

	// assertRStackInvariant checks for the R stack invariant, that truth degrees never increase
	// as we move from the bottom to the top, using Formula (71), p. 227
	assertRStackInvariant := func() {
		for j := 1; j < E; j++ {
			if VAL[R[j-1]>>1]|1 < VAL[R[j]>>1] {
				dump()
				log.Fatal("assertion failed: violation of the R stack invariant")
			}
		}
	}

	// binary_propogation uses a simple breadth-first search procedure
	// to propagate the binarary consequences of a literal l in context T.
	// Returns false if no conflict, true if there is conflict.
	// Formula (62), p. 221
	// @note binary_propogation()
	binary_propagation := func(l int) bool {

		if debug {
			log.Printf("  binary_propagation l=%d, t=%s", l, truth(T))
			assertRStackInvariant()
		}

		H := E

		// Take account of l
		x := l >> 1
		if VAL[x] >= T {

			// l is fixed in context T
			if VAL[x]&1 == l&1 {
				// l is fixed true, do nothing
				return false

			} else {
				// l is fixed false, goto CONFLICT
				return true
			}
		}

		VAL[x] = T + (l & 1)
		R[E] = l
		E += 1

		for H < E {
			l = R[H]
			H += 1

			// For each l' in BIMP(l)
			for j := 0; j < BSIZE[l]; j++ {
				lp := BIMP[l][j]
				xp := lp >> 1

				// Take account of l'
				if VAL[xp] >= T {

					// l' is fixed in context T
					if VAL[xp]&1 == lp&1 {
						// l' is fixed true, do nothing

					} else {
						// l' is fixed false, goto CONFLICT
						return true
					}
				} else {
					VAL[xp] = T + (lp & 1)
					R[E] = lp
					E += 1
				}
			}
		}

		if debug {
			assertRStackInvariant()
		}

		return false
	}

	// lvisit prepares the solution
	lvisit := func() []int {
		solution := make([]int, n)

		// Convert the literals from internal back to external format
		for i := 0; i < n; i++ {
			l := R[i]
			solution[(l>>1)-1] = (l & 1) ^ 1
		}
		if debug {
			log.Printf("visit solution=%v (%v)", solution[:nOrig], solution)
		}

		return solution[:nOrig]
	}

	// appendBimp adds u to BIMP[l]
	appendBimp := func(l, u int) {

		// Update private stamp IST, if necessary. Formula (63)
		if IST[l] != ISTAMP {
			IST[l] = ISTAMP
			if I == len(ISTACK) {
				ISTACK = append(ISTACK, [2]int{l, BSIZE[l]})
			} else {
				ISTACK[I][0] = l
				ISTACK[I][1] = BSIZE[l]
			}
			I += 1
		}

		// Append u to BIMP[l]
		if BSIZE[l] == len(BIMP[l]) {
			BIMP[l] = append(BIMP[l], u)
		} else {
			BIMP[l][BSIZE[l]] = u
		}
		BSIZE[l] += 1
	}

	//
	// @note L1 [Initialize.]
	//

	// If this is a kSAT problem where k > 3 then either convert to 3SAT or use optional "big" clauses.
	nOrig = n

	sat3, nSat3, clausesSat3 := Sat3(n, clauses)

	if !sat3 {
		if !optionsL.SuppressBigClauses {
			bigClauses = true
		} else {
			n = nSat3
			clauses = clausesSat3
		}
	}

	initialize()

	if debug {
		log.Printf("L1. Initialize")
	}

	// Convert the literals in each clause from external to internal format
	clausesInternal := make(SatClauses, len(clauses))
	for i, clause := range clauses {
		clausesInternal[i] = make(SatClause, len(clause))
		for j, k := range clause {
			if k < 0 {
				clausesInternal[i][j] = -2*k + 1
			} else {
				clausesInternal[i][j] = 2 * k
			}
		}
	}
	clauses = clausesInternal

	//
	// Record all unit clauses as forced variable values at depth 0
	//
	FORCE = make([]int, 2*n+2)
	U = 0
	for _, clause := range clauses {
		if len(clause) == 1 {
			l := clause[0]

			// Look for a contradiction
			for k := 0; k < U; k++ {
				if l^1 == FORCE[k] {
					// A contradiction
					if debug {
						log.Printf("L1. Found a unit clause contradiction")
					}
					return false, nil
				}
			}

			FORCE[U] = l
			U += 1

		}
	}

	//
	// Record all binary clauses in the BIMP array
	//
	BIMP = make([][]int, 2*n+2)
	for l := 2; l <= 2*n+1; l++ {
		BIMP[l] = make([]int, 4)
	}
	BSIZE = make([]int, 2*n+2)

	// Insert binary clauses into BIMP
	for _, clause := range clauses {
		// Check for clause of length 2
		if len(clause) == 2 {
			u, v := clause[0], clause[1]

			if BSIZE[u^1] == len(BIMP[u^1]) {
				BIMP[u^1] = append(BIMP[u^1], v)
			} else {
				BIMP[u^1][BSIZE[u^1]] = v
			}
			BSIZE[u^1] += 1

			if BSIZE[v^1] == len(BIMP[v^1]) {
				BIMP[v^1] = append(BIMP[v^1], u)
			} else {
				BIMP[v^1][BSIZE[v^1]] = u
			}
			BSIZE[v^1] += 1
		}
	}

	if bigClauses {
		//
		// Record all k > 2 clauses in the KINX and CINX arrays
		// @note L1 - KINX, CINX
		//

		KINX = make([][]int, 2*n+2)
		KSIZE = make([]int, 2*n+2)

		CINX = make([][]int, 0)
		CSIZE = make([]int, 0)
		CTHETA = make([]int, 0)

		// Initialize CINX and CSIZE
		for _, clause := range clauses {
			// Check for clause of length > 2
			csize := len(clause)
			if csize > 2 {
				CINX = append(CINX, make([]int, 0, len(clause)))
				i := len(CINX) - 1
				for _, l := range clause {
					CINX[i] = append(CINX[i], l)
				}
				CSIZE = append(CSIZE, csize)

				// Compute 𝜃 value for this c
				theta := 0
				if csize > 32 {
					theta = int(math.Round(float64(csize) * 25 / 64))
				}
				CTHETA = append(CTHETA, theta)
			}
		}

		// Initialize KINX and KSIZE
		maxKSize := 0

		// ∀ literal l
		for l := 2; l <= 2*n+1; l++ {

			// ∀ big clause c
			for c, clause := range CINX {

				// Look for l in c
				for i := 0; i < len(clause); i++ {
					u := clause[i]
					if l == u {
						// Found l in clause c
						KINX[l] = append(KINX[l], c)
						KSIZE[l] += 1
						if KSIZE[l] > maxKSize {
							maxKSize = KSIZE[l]
						}
						break
					}
				}
			}
		}

		uvStack = make([][2]int, 0, maxKSize)

	} else {

		//
		// Record all ternary clauses in the TIMP array
		// @note L1 - TIMP
		//
		TIMP = make([]int, 2*n+2)
		TSIZE = make([]int, 2*n+2)

		// Get the values of TIMP[l] and TSIZE[l] for each l
		for l := 2; l <= 2*n+1; l++ {
			// Look for clauses containing this literal
			for _, clause := range clauses {
				// Check for clause of length 3
				if len(clause) == 3 {
					u, v, w := clause[0], clause[1], clause[2]

					if l == u^1 || l == v^1 || l == w^1 {
						// Found l in this clause
						if TIMP[l] == 0 {
							// This is the first clause in the list for l
							TIMP[l] = len(TIMP)
						}
						TIMP = append(TIMP, 0, 0)
						TSIZE[l] += 1
					}
				}
			}
		}

		// Add each clause to TIMP and set their LINK values
		LINK = make([]int, len(TIMP))
		tindex := make([]int, 2*n+2) // tindex[l] is the index for next insertion point in TIMP[l]

		for _, clause := range clauses {
			// Check for clause of length 3
			if len(clause) == 3 {
				u, v, w := clause[0], clause[1], clause[2]

				p = TIMP[u^1] + tindex[u^1]
				TIMP[p] = v
				TIMP[p+1] = w
				tindex[u^1] += 2

				pp = TIMP[v^1] + tindex[v^1]
				TIMP[pp] = w
				TIMP[pp+1] = u
				tindex[v^1] += 2

				ppp = TIMP[w^1] + tindex[w^1]
				TIMP[ppp] = u
				TIMP[ppp+1] = v
				tindex[w^1] += 2

				LINK[p] = pp
				LINK[pp] = ppp
				LINK[ppp] = p
			}
		}
	}

	// Configure initial permutation of the "free variable" list, that is,
	// not fixed in context RT. A variable becomes fixed by swapping it to the
	// end of the free list and decreasing N; then we can free it later by
	// simply increasing N, without swapping.
	VAR = make([]int, n)
	INX = make([]int, n+1)
	for k = 1; k <= n; k++ {
		VAR[k-1] = k
		INX[k] = k - 1
	}
	N = n

	d = 0
	F = 0

	ISTAMP = 0
	BSTAMP = 0
	IST = make([]int, 2*n+2)
	BST = make([]int, 2*n+2)
	ISTACK = make([][2]int, 1024) // Grow dynamically, when needed
	I = 0

	DEC = make([]int, n+1)
	BACKF = make([]int, n+1)
	BACKI = make([]int, n+1)
	BACKL = make([]int, n+1)
	BRANCH = make([]int, n+1)

	VAL = make([]int, n+1)
	R = make([]int, n+1)

	if debug && stats.Verbosity > 0 {
		dump()
	}

	//
	// @note L2 [New node.]
	//
L2:
	if debug {
		log.Printf("L2. New node")
	}

	BRANCH[d] = -1 // No decision yet
	BACKL[d] = F

	if progress && stats.Delta != 0 && stats.Nodes%stats.Delta == 0 {
		showProgress()
	}
	if debug && stats.Verbosity > 0 {
		dump()
	}

	if U > 0 {
		goto L5
	}

	//
	// Algorithm X
	//

	// Iterate over each R stack entry, checking for contradictions
	for i := 0; i < E; i++ {
		l := R[i]

		// Iterate over l's BIMP table
		for j := 0; j < BSIZE[l]; j++ {
			lp := BIMP[l][j]

			// Look for a conflict between the BIMP table entry and an R stack entry
			for k := 0; k < E; k++ {
				if i != k {
					lpp := R[k]

					if lp^1 == lpp {
						// A contradiction
						if debug && stats.Verbosity > 0 {
							dump()
							log.Printf("L2. BIMP table for %d in R contains %d, which contradicts %d in R", l, lp, lpp)
						}
						goto L15
					}
				}
			}
		}
	}

	if F == n {
		// All variables are fixed, visit the solution

		if debug {
			log.Println("L2. [Success!]")
		}

		if stats != nil {
			stats.Solutions++
		}

		if progress {
			showProgress()
		}

		return true, lvisit()
	}

	// Choose whatever literal happens to be first in the current list
	// of free variables.
	x = VAR[0]
	l = 2 * x

	stats.Levels[d]++
	stats.Nodes++
	if d > stats.MaxLevel {
		stats.MaxLevel = d
	}

	if debug {
		log.Printf("  Selected d=%d, branch=%v, l=%d from free variable list", d, BRANCH[0:d], l)
		if stats.Verbosity > 0 {
			dump()
		}
	}

	//
	// @note L3 [Choose l.]
	//
L3:
	if debug {
		log.Printf("L3. Choose l")
	}

	if l == 0 {
		d += 1
		goto L2
	}

	if debug {
		log.Printf("  d=%d, l=%d", d, l)
	}

	DEC[d] = l
	BACKF[d] = F
	BACKI[d] = I
	BRANCH[d] = 0 // We are trying l

	//
	// @note L4 [Try l.]
	//
L4:

	if debug {
		log.Printf("L4. Try l")
	}

	U = 1
	FORCE[0] = l

	//
	// @note L5 [Accept near truths.]
	//
L5:
	if debug {
		log.Printf("L5. Accept near truths")
	}

	T = nt
	G, E = F, F
	ISTAMP += 1
	CONFLICT = 11 // L11

	// Iterate over each l in the FORCE stack
	for i := 0; i < U; i++ {
		if debug && stats.Verbosity > 0 && i == 0 {
			log.Printf("State before beginning binary_propagation")
			dump()
		}

		l := FORCE[i]

		// Perform the binary propogation routine
		if binary_propagation(l) {
			// There was a conflict
			switch CONFLICT {
			case 11:
				goto L11
			default:
				log.Panicf("Unknown value of CONFLICT: %d", CONFLICT)
			}

		}
	}

	U = 0

	if debug && stats.Verbosity > 0 {
		dump()
	}
	//
	// @note L6 [Choose a nearly true L.]
	//
L6:
	if debug {
		log.Printf("L6. Choose a nearly true L")
	}

	// At this point the stacked literals R_k are "really true" for 0 <= k < G,
	// and "nearly true" for G <= k < E. We want them all to be really true.

	if debug {
		// assertion
		for k := 0; k < E; k++ {
			l := R[k]
			x := l >> 1

			if k < G && VAL[x]&(^1) != rt {
				log.Panicf("assertion failed: variable {%d}=%s is not RT at L6", x, truth(VAL[x]))

			} else if k >= G && VAL[x]&(^1) != nt {
				log.Panicf("assertion failed: variable {%d}=%s is not NT at L6", x, truth(VAL[x]))
			}
		}
	}

	if G == E {
		// No nearly true literals
		goto L10
	}

	L = R[G]
	G += 1

	//
	// @note L7 [Promote L to real truth.]
	//

	if debug {
		log.Printf("L7. Promote L=%d to real truth", L)
	}

	X = L >> 1
	VAL[X] = rt + L&1

	// Remove variable X from the free list (Exercise 137 (a))
	N = n - G
	x = VAR[N]
	j = INX[X]
	VAR[j] = x
	INX[x] = j
	VAR[N] = X
	INX[X] = N

	if bigClauses {
		// Remove variable X from all CINX/KINX clauses (Exercise 143)
		// @note L7 - KINX,CINX

		//
		// Deactivate all of the active big clauses that contain L
		//

		BACKKSIZE = append(BACKKSIZE, KSIZE[L])

		// ∀ c ∈ KINX[L]
		for KSIZE[L] > 0 {
			c := KINX[L][0]

			// ∀ u ∈ CINX[c]
			swapFreeLiterals := CSIZE[c] < CTHETA[c] // Swap free literals to beginning of CINX[c]
			freeLiterals := 0                        // Number of free literals found
			for i, u := range CINX[c] {

				// Check if u is a free literal
				if u == L || VAL[u>>1] < rt {

					// Swap c out of u's clause list
					s := KSIZE[u] - 1
					KSIZE[u] = s
					for t := 0; t < s; t++ {
						if KINX[u][t] == c {
							KINX[u][t] = KINX[u][s]
							KINX[u][s] = c
							break
						}
					}

					// Swap free literal u to the beginning of CINX[c]
					if swapFreeLiterals {
						if i > freeLiterals {
							CINX[c][freeLiterals], CINX[c][i] = CINX[c][i], CINX[c][freeLiterals]
						}
					}
					freeLiterals++
					if freeLiterals == CSIZE[c] {
						break
					}
				}
			}
		}

		//
		// Update clauses for which L has become false
		//

		// empty the stack
		uvStack = uvStack[:0]

		// ∀ c ∈ KINX[¬L]
		for i := 0; i < KSIZE[L^1]; i++ {
			c := KINX[L^1][i]
			s := CSIZE[c] - 1
			CSIZE[c] = s

			// If s > 2, don't bother moving the free literals.  We'll simply
			// search for the last two free literals when needed when the size
			// reaches 2.
			if s == 2 {
				// Find the two free literals (u, v) ∈ CINX[c]
				// ∀ u ∈ CINX[c]
				i := 0 // index into CINX[c], 0 for u, 1 for v
				for j, u := range CINX[c] {

					// Determine if u is in the VAR free list
					if INX[u>>1] < N {

						// Swap (u, v) into the first positions of CINX[c],
						// if not already there
						if i != j {
							CINX[c][j] = CINX[c][i]
							CINX[c][i] = u
						}
						if i == 1 {
							break
						} else {
							i += 1
						}
					}

				}

				// Put (u, v) onto a temporary stack
				uvStack = append(uvStack, [2]int{CINX[c][0], CINX[c][1]})

				// Swap c out of lists u and v
				for _, u := range CINX[c][0:2] {
					s := KSIZE[u] - 1
					KSIZE[u] = s
					for t := 0; t < s; t++ {
						if KINX[u][t] == c {
							KINX[u][t] = KINX[u][s]
							KINX[u][s] = c
							break
						}
					}
				}
			}

		}

	} else {
		// Remove variable X from all TIMP pairs (Exercise 137 (a))
		// @note L7 - TIMP

		for _, l := range []int{2 * X, 2*X + 1} {

			// For each pair in TIMP[l]
			for i := 0; i < TSIZE[l]; i++ {
				p = TIMP[l] + 2*i
				u, v := TIMP[p], TIMP[p+1]

				pp = LINK[p]
				ppp = LINK[pp]

				// Decrease the size of TIMP[u^1] by 1
				s := TSIZE[u^1] - 1
				TSIZE[u^1] = s
				t := TIMP[u^1] + 2*s

				if pp != t {
					// Swap pairs, if t did not point to the last pair in TIMP[u^1]
					up, vp := TIMP[t], TIMP[t+1]
					q := LINK[t]
					qp := LINK[q]
					LINK[qp], LINK[p] = pp, t
					TIMP[pp], TIMP[pp+1] = up, vp
					LINK[pp] = q
					TIMP[t], TIMP[t+1] = v, l^1
					LINK[t] = ppp
					pp = t
				}

				// Decrease the size of TIMP[v^1] by 1
				s = TSIZE[v^1] - 1
				TSIZE[v^1] = s
				t = TIMP[v^1] + 2*s

				if ppp != t {
					// Swap pairs, if t did not point to the last pair in TIMP[v^1]
					up, vp := TIMP[t], TIMP[t+1]
					q := LINK[t]
					qp := LINK[q]
					LINK[qp], LINK[pp] = ppp, t
					TIMP[ppp], TIMP[ppp+1] = up, vp
					LINK[ppp] = q
					TIMP[t], TIMP[t+1] = l^1, u
					LINK[t] = p
				}
			}
		}
	}

	// ∀ (u,v) ready for BIMP consideration
	for i := 0; true; i++ {
		if debug && stats.Verbosity > 0 {
			dump()
		}

		var u, v int

		if bigClauses {
			// Get (u, v) from uvStack
			if i == len(uvStack) {
				break
			}
			u, v = uvStack[i][0], uvStack[i][1]
		} else {
			// Get (u, v) from TIMP[L]
			if i == TSIZE[L] {
				break
			}
			p := TIMP[L] + 2*i
			u, v = TIMP[p], TIMP[p+1]
		}

		//
		// @note L8 [Consider u or v.]
		//

		if debug {
			log.Printf("L8. Consider u=%d or v=%d", u, v)
		}

		// We have deduced that u or v must be true; five cases arise.

		uFixed := VAL[u>>1] >= T
		uFixedTrue := uFixed && VAL[u>>1]&1 == u&1

		if uFixedTrue {
			// Case 1. u or v is fixed true, do nothing
			if debug && stats.Verbosity > 0 {
				log.Printf(" Case 1. u=%d is fixed true", u)
			}
			continue
		}

		vFixed := VAL[v>>1] >= T
		vFixedTrue := vFixed && VAL[v>>1]&1 == v&1

		if vFixedTrue {
			// Case 1. u or v is fixed true, do nothing
			if debug && stats.Verbosity > 0 {
				log.Printf(" Case 1. v=%d is fixed true", v)
			}
			continue
		}

		uFixedFalse := uFixed && VAL[(u^1)>>1]&1 == (u^1)&1
		vFixedFalse := vFixed && VAL[(v^1)>>1]&1 == (v^1)&1

		if uFixedFalse && vFixedFalse {

			// Case 2. u and v are fixed false
			if debug && stats.Verbosity > 0 {
				log.Printf(" Case 2. u=%d and v=%d are fixed false; CONFLICT", u, v)
			}
			switch CONFLICT {
			case 11:
				goto L11
			default:
				log.Panicf("Unknown value of CONFLICT: %d", CONFLICT)
			}

		} else if uFixedFalse && !vFixed {

			// Case 3. u is fixed false but v isn't fixed
			if debug && stats.Verbosity > 0 {
				log.Printf(" Case 3. u=%d is fixed false but v=%d isn't fixed", u, v)
			}

			if binary_propagation(v) {
				switch CONFLICT {
				case 11:
					goto L11
				default:
					log.Panicf("Unknown value of CONFLICT: %d", CONFLICT)
				}
			}

		} else if vFixedFalse && !uFixed {

			// Case 4. v is fixed false but u isn't fixed
			if debug && stats.Verbosity > 0 {
				log.Printf(" Case 4. v=%d is fixed false but u=%d isn't fixed", v, u)
			}
			if binary_propagation(u) {
				switch CONFLICT {
				case 11:
					goto L11
				default:
					log.Panicf("Unknown value of CONFLICT: %d", CONFLICT)
				}
			}

		} else {

			// Case 5. Neither u nor v is fixed
			if debug && stats.Verbosity > 0 {
				log.Printf(" Case 5. Neither u=%d nor v=%d is fixed", u, v)
			}

			//
			// @note L9 [Exploit u or v.]
			//
			if debug {
				log.Printf("L9. Exploit u=%d or v=%d", u, v)
			}

			if optionsL.CompensationResolvants {

				// Compensation Resolvants (Exercise 139)
				// If w ∈ BIMP[v], the binary clause u ⋁ v implies the binary clause u ⋁ w,
				// because we can resolve u ⋁ v with ¬u ⋁ w.

				for _, pair := range [2][]int{{u, v}, {v, u}} {
					u, v := pair[0], pair[1]

					BSTAMP += 1

					// Mark everything implied true by ¬u with current BSTAMP
					// ∀ l ∈ ¬u ∪ BIMP(¬u): BST[l] ← BSTAMP
					BST[u^1] = BSTAMP
					for i := 0; i < BSIZE[u^1]; i++ {
						l := BIMP[u^1][i]
						BST[l] = BSTAMP
					}

					// If ¬u implies neither v nor ¬v
					if BST[v^1] != BSTAMP && BST[v] != BSTAMP {

						// Iterate over w ∈ BIMP[v]
						for i := 0; i < BSIZE[v]; i++ {
							w := BIMP[v][i]

							if VAL[w>>1] >= nt {
								// do nothing, w is fixed in NT
								if VAL[w>>1]&1 != w&1 {
									log.Fatalf("assertion failed: violation w=%d if fixed, must be fixed true", w)
								}

							} else if BST[w^1] == BSTAMP {
								// ¬u implies both w and ¬w, so let's try and propagate u
								if binary_propagation(u) {
									switch CONFLICT {
									case 11:
										goto L11
									default:
										log.Panicf("Unknown value of CONFLICT: %d", CONFLICT)
									}
								}
								break

							} else if BST[w] != BSTAMP {
								// ¬u did not already imply w, so add new binary clause u ∨ w
								appendBimp(u^1, w) // ¬u implies w
								appendBimp(w^1, u) // ¬w implies u
							}

						}
					}
				}
			}

			// Check for v or ¬v in BIMP[¬u]
			var vInBimp, notvInBimp bool
			for i := 0; i < BSIZE[u^1]; i++ {
				if BIMP[u^1][i] == v {
					vInBimp = true
					break
				}
				if BIMP[u^1][i] == v^1 {
					notvInBimp = true
					break
				}
			}

			if notvInBimp {
				// ¬v ∈ BIMP[¬u], so select u as true
				if binary_propagation(u) {
					switch CONFLICT {
					case 11:
						goto L11
					default:
						log.Panicf("Unknown value of CONFLICT: %d", CONFLICT)
					}
				}
			} else if vInBimp {
				// v ∈ BIMP[¬u]
				// do nothing, we already have the clause u or v
			} else {

				// Check ¬u in BIMP[¬v]
				var notuInBimp bool
				for i := 0; i < BSIZE[v^1]; i++ {
					if BIMP[v^1][i] == u^1 {
						notuInBimp = true
						break
					}
				}

				if notuInBimp {
					// ¬u ∈ BIMP[¬v], so select v as true
					if binary_propagation(v) {
						switch CONFLICT {
						case 11:
							goto L11
						default:
							log.Panicf("Unknown value of CONFLICT: %d", CONFLICT)
						}
					}
				} else {
					// Accept new binary clause (u, v), could be either
					appendBimp(u^1, v)
					appendBimp(v^1, u)
				}

			}

		}

	}

	goto L6

	//
	// @note L10 [Accept real truths.]
	//
L10:
	if debug {
		log.Printf("L10. Accept real truths")
	}

	F = E

	if BRANCH[d] >= 0 {
		d += 1
		if debug {
			log.Printf("  branch[%d]=%d, incremented d to %d, going to L2", d-1, BRANCH[d-1], d)
		}
		goto L2
	} else if d > 0 {
		// Does not occur for L^0, because we never "don't decide"
		log.Panic("Can't get here for L^0")
		if debug {
			log.Printf("  branch[%d]=%d and d=%d > 0, going to L3", d, BRANCH[d], d)
		}
		goto L3
	} else { // d == 0
		// Only occurs if there are unit clauses in the input
		if debug {
			log.Printf("  branch[%d]=%d and d=0, going to L2", d, BRANCH[d])
		}
		goto L2
	}

	//
	// @note L11 [Unfix near truths.]
	//
L11:

	if debug {
		log.Printf("L11. Unfix near truths")
	}

	for E > G {
		E -= 1
		VAL[R[E]>>1] = 0
	}

	//
	// @note L12 [Unfix real truths.]
	//
L12:
	if debug {
		log.Printf("L12. Unfix real truths, E=%d, F=%d", E, F)
	}

	if debug && stats.Verbosity > 0 {
		dump()
	}

	for E > F {
		// Implicitly restore X to the free list because N + E = n
		// (Exercise 137)
		E -= 1
		L = R[E]
		X = L >> 1

		if bigClauses {
			// Restore variable X to all CINX/KINX clauses (Exercise 143)
			// @note L12 - KINX,CINX

			//
			// Update clauses for which L has ceased being false
			//

			// ∀ c ∈ KINX[¬L] (reverse order from L7)
			for i := KSIZE[L^1] - 1; i >= 0; i-- {
				c := KINX[L^1][i]
				s := CSIZE[c]
				CSIZE[c] = s + 1

				if s == 2 {
					// Swap c back into the clauses list of (u, v)
					for _, u := range CINX[c][0:2] {
						KSIZE[u] += 1
					}
				}
			}

			//
			// Reactivate all of the active big clauses that contain L
			//

			// Pop last value of KSIZE[L]
			backksize, BACKKSIZE = BACKKSIZE[len(BACKKSIZE)-1], BACKKSIZE[:len(BACKKSIZE)-1]

			// ∀ c ∈ KINX[L] (reverse order from L7)
			for i := 0; i < backksize; i++ {
				c := KINX[L][i]

				// ∀ u ∈ CINX[c] (reverse order from L7)
				for j := len(CINX[c]) - 1; j >= 0; j-- {
					u := CINX[c][j]

					// Check if u is a free literal
					if u == L || VAL[u>>1] < rt {

						// Swap c back into u's clause list
						KSIZE[u] += 1
					}
				}
			}

			// TODO: ParamILS advises changing 𝛼 from 3.5 to 0.001(!) in (195).

		} else {
			// Reactivate the TIMP pairs that involve X
			// (Exercise 137)
			// @note L12 - TIMP
			for _, l = range []int{2*X + 1, 2 * X} {
				for i := TSIZE[l] - 1; i >= 0; i-- {
					p := TIMP[l] + 2*i
					u, v := TIMP[p], TIMP[p+1]

					TSIZE[v^1] += 1
					TSIZE[u^1] += 1
				}
			}
		}

		VAL[X] = 0
	}

	if debug && stats.Verbosity > 0 {
		dump()
	}

	//
	// @note L13 [Downdate BIMPs.]
	//

	if debug {
		log.Printf("L13. Downdate BIMPs")
	}

	if BRANCH[d] >= 0 {
		for I > BACKI[d] {
			I -= 1
			l, s := ISTACK[I][0], ISTACK[I][1]
			BSIZE[l] = s
		}
	}

	//
	// @note L14 [Try again?]
	//

	if debug {
		log.Printf("L14. Try again?")
	}

	// We've discovered that DEC[d] doesn't work
	if BRANCH[d] == 0 {
		l = DEC[d] ^ 1
		DEC[d] = l
		BRANCH[d] = 1 // l didn't work out, so try ^l

		if debug {
			log.Printf("  Trying again, d=%d, branch=%v, l=%d", d, BRANCH[0:d], l)
		}

		goto L4
	}

	//
	// @note L15 [Backtrack.]
	//
L15:
	if debug {
		log.Printf("L15. Backtrack")
	}

	if d == 0 {
		// Terminate unsuccessfully
		return false, nil
	}

	d -= 1
	E = F
	F = BACKF[d]
	goto L12
}
