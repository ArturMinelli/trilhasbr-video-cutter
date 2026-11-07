package tracking

import "math"

// assign solves the rectangular assignment problem minimizing total cost.
// Costs are clamped to maxCost first, and pairs whose cost reaches maxCost are
// dropped, so over-threshold entries behave as "infeasible" (same effect as lapjv cost_limit).
// Returns matched (row, col) pairs ordered by row, plus unmatched rows and cols.
func assign(cost [][]float64, nCols int, maxCost float64) (pairs [][2]int, rowsLeft, colsLeft []int) {
	n := len(cost)
	rowOf := make([]int, nCols) // col -> row, -1 if free
	for j := range rowOf {
		rowOf[j] = -1
	}
	if n > 0 && nCols > 0 {
		c := make([][]float64, n)
		for i := range cost {
			c[i] = make([]float64, nCols)
			for j, v := range cost[i] {
				c[i][j] = math.Min(v, maxCost)
			}
		}
		for j, i := range solve(c, nCols) {
			rowOf[j] = i
		}
	}
	rowMatched := make([]bool, n)
	for j, i := range rowOf {
		if i >= 0 && cost[i][j] < maxCost {
			rowMatched[i] = true
		} else {
			rowOf[j] = -1
			colsLeft = append(colsLeft, j)
		}
	}
	for i := 0; i < n; i++ {
		if !rowMatched[i] {
			rowsLeft = append(rowsLeft, i)
		}
	}
	for j, i := range rowOf {
		if i >= 0 {
			pairs = append(pairs, [2]int{i, j})
		}
	}
	// ponytail: pairs ordered by col above; sort by row for determinism.
	for a := 1; a < len(pairs); a++ {
		for b := a; b > 0 && pairs[b][0] < pairs[b-1][0]; b-- {
			pairs[b], pairs[b-1] = pairs[b-1], pairs[b]
		}
	}
	return
}

// solve runs the O(n²m) potentials-based Hungarian algorithm and returns col -> row (-1 free).
// Handles n > m by assigning transposed.
func solve(c [][]float64, m int) []int {
	n := len(c)
	if n > m {
		t := make([][]float64, m)
		for j := range t {
			t[j] = make([]float64, n)
			for i := 0; i < n; i++ {
				t[j][i] = c[i][j]
			}
		}
		rowToCol := solve(t, n) // for transposed: col(i of orig) -> row(j of orig)
		out := make([]int, m)
		for j := range out {
			out[j] = -1
		}
		for i, j := range rowToCol {
			if j >= 0 {
				out[j] = i
			}
		}
		return out
	}
	inf := math.Inf(1)
	u := make([]float64, n+1)
	v := make([]float64, m+1)
	p := make([]int, m+1)
	way := make([]int, m+1)
	for i := 1; i <= n; i++ {
		p[0] = i
		j0 := 0
		minv := make([]float64, m+1)
		used := make([]bool, m+1)
		for j := range minv {
			minv[j] = inf
		}
		for {
			used[j0] = true
			i0, delta, j1 := p[j0], inf, 0
			for j := 1; j <= m; j++ {
				if used[j] {
					continue
				}
				cur := c[i0-1][j-1] - u[i0] - v[j]
				if cur < minv[j] {
					minv[j], way[j] = cur, j0
				}
				if minv[j] < delta {
					delta, j1 = minv[j], j
				}
			}
			for j := 0; j <= m; j++ {
				if used[j] {
					u[p[j]] += delta
					v[j] -= delta
				} else {
					minv[j] -= delta
				}
			}
			j0 = j1
			if p[j0] == 0 {
				break
			}
		}
		for j0 != 0 {
			j1 := way[j0]
			p[j0] = p[j1]
			j0 = j1
		}
	}
	out := make([]int, m)
	for j := 1; j <= m; j++ {
		out[j-1] = p[j] - 1
	}
	return out
}
