package util

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
)

// Sample randomly selects k integers from the range [0, max).
// Given that Sample's algorithm is based on the Fisher-Yates shuffle, it's complexity is O(k).
func Sample(k, max int) (sampled []int) {
	if k >= max {
		for i := 0; i < max; i++ {
			sampled = append(sampled, i)
		}
		return
	}

	swapped := make(map[int]int, k)
	for i := 0; i < k; i++ {
		// generate a random number from [i, max)
		r := rand.Intn(max-i) + i

		// swapped[i], swapped[r] = swapped[r], swapped[i]
		vr, ok := swapped[r]
		if ok {
			sampled = append(sampled, vr)
		} else {
			sampled = append(sampled, r)
		}
		vi, ok := swapped[i]
		if ok {
			swapped[r] = vi
		} else {
			swapped[r] = i
		}
	}
	return
}

func JSONReq3(method, urlStr string, res interface{}) (*http.Response, []byte, error) {
	return JSONReq6(method, urlStr, nil, nil, http.DefaultClient, res)
}

func JSONReq5(method, urlStr string, body io.Reader, header http.Header, res interface{}) (*http.Response, []byte, error) {
	return JSONReq6(method, urlStr, body, header, http.DefaultClient, res)
}

func JSONReq6(method, urlStr string, body io.Reader, header http.Header, c *http.Client, res interface{}) (*http.Response, []byte, error) {
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return nil, nil, err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if res == nil {
		return resp, b, nil
	}
	err = json.Unmarshal(b, res)
	if err != nil {
		return nil, nil, fmt.Errorf(`json unmarshal error "%v" for body: %s`, err, b)
	}
	return resp, b, nil
}
