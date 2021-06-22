package fwncs

import (
	"errors"
	"math/rand"
	"net/http"
	"reflect"
	"runtime"
	"time"
)

func NameOfFunction(f interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
}

func bodyAllowedForStatus(status int) bool {
	switch {
	case status >= 100 && status <= 199:
		return false
	case status == http.StatusNoContent:
		return false
	case status == http.StatusNotModified:
		return false
	}
	return true
}

func lastChar(str string) uint8 {
	if str == "" {
		panic("string can't be 0")
	}
	return str[len(str)-1]
}

var MinMaxError = errors.New("Min cannot be greater than max.")

var randGenerator = func(max float64) float64 {
	rand.Seed(time.Now().UnixNano())
	r := rand.Float64() * max
	return r
}

type Choice struct {
	Weight float64
	Item   interface{}
}

type Choices []Choice

func (c Choices) GetOne() Choice {
	w := make([]float64, len(c))
	for i, choice := range c {
		w[i] = choice.Weight
	}
	idx := weightedChoiceOne(len(c), w)
	return c[idx]
}

func (c Choices) Get(n int) []Choice {
	length := len(c)
	weight := make([]float64, length)
	for i, choice := range c {
		weight[i] = choice.Weight
	}
	v, _ := weightedChoice(length, n, weight)
	results := make(Choices, 0, len(v))
	for i := 0; i < len(v); i++ {
		results[i] = c[v[i]]
	}
	return results
}

func weightedChoiceOne(v int, w []float64) int {
	// v を slice　に変換
	vs := make([]int, 0, v)
	for i := 0; i < v; i++ {
		vs = append(vs, i)
	}

	// weightの合計値を計算
	var sum float64
	for _, v := range w {
		sum += v
	}

	// weightの合計から基準値をランダムに選ぶ
	r := randGenerator(sum)

	// weightを基準値から順に引いていき、0以下になったらそれを選ぶ
	for j, v := range vs {
		r -= w[j]
		if r < 0 {
			return v
		}
	}
	// should return error...
	return 0
}

func weightedChoice(v, size int, w []float64) ([]int, error) {
	// convert v to slice.
	vs := make([]int, 0, v)
	for i := 0; i < v; i++ {
		vs = append(vs, i)
	}

	var sum float64
	for _, v := range w {
		sum += v
	}

	result := make([]int, 0, size)
	for i := 0; i < size; i++ {

		r := randGenerator(sum)

		for j, v := range vs {
			r -= w[j]
			if r < 0 {
				result = append(result, v)

				sum -= w[j]

				// delete choiced item.
				// https://github.com/golang/go/wiki/SliceTricks#delete
				w = append(w[:j], w[j+1:]...)
				vs = append(vs[:j], vs[j+1:]...)

				break
			}
		}
	}
	return result, nil
}
