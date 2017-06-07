package main

import (
	"crypto/rand"
	"fmt"
	. "github.com/sendgrid/go-solr"
	"io"
	"time"
)

var (
	solrHttpRetrier SolrHTTP
	solrZk          SolrZK
	locator         SolrLocator
)

func init() {
	var err error
	solrZk = NewSolrZK("zk:2181", "solr", "solrtest")
	locator = solrZk.GetSolrLocator()
	err = solrZk.Listen()
	if err != nil {
		panic(err)
	}
	https, err := solrZk.UseHTTPS()
	if err != nil {
		panic(err)
	}
	solrHttp, err := NewSolrHTTP(https, "solrtest", User("solr"), Password("admin"), MinRF(2))
	if err != nil {
		panic(err)
	}
	solrHttpRetrier = NewSolrHttpRetrier(solrHttp, 5, 100*time.Millisecond)

}
func main() {
	const limit int = 100 * 1000
	numFound, err := run(limit, "/1")
	if err != nil {
		panic(err)
	}
	if limit != int(numFound) {
		panic(fmt.Sprintf("limit did not match what was found %d=%d", limit, numFound))
	}
	fmt.Println(fmt.Sprintf("runner done %d", numFound))
	numFound, err = run(limit, "")
	if err != nil {
		panic(err)
	}
	if limit != int(numFound) {
		panic(fmt.Sprintf("limit did not match what was found %d=%d", limit, numFound))
	}
	fmt.Println(fmt.Sprintf("runner done %d", numFound))
}

func run(limit int, bits string) (uint32, error) {
	uuid, _ := newUUID()
	for i := 0; i < limit; i++ {
		shardKey := fmt.Sprintf("mycrazyshardkey%d", i%10)
		iterationId, _ := newUUID()
		doc := map[string]interface{}{
			"id":         fmt.Sprintf("%s%s!rando%s", shardKey, bits, iterationId),
			"email":      "rando" + iterationId + "@sendgrid.com",
			"first_name": "tester" + iterationId,
			"last_name":  uuid,
		}
		all, err := locator.GetLeadersAndReplicas(doc["id"].(string))
		if err != nil {
			panic(err)
		}
		if i < limit-1 {
			err := solrHttpRetrier.Update(all, true, doc, Commit(false))
			if err != nil {
				panic(err)
			}
		} else {
			err := solrHttpRetrier.Update(all, true, doc, Commit(true))
			if err != nil {
				panic(err)
			}
		}

	}
	replicas, err := locator.GetReplicaUris()

	if err != nil {
		panic(err)
	}
	r, err := solrHttpRetrier.Read(replicas, Query("*:*"), FilterQuery("last_name:"+uuid), Rows(uint32(limit)))
	return r.Response.NumFound, err

}
func newUUID() (string, error) {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(rand.Reader, uuid)
	if n != len(uuid) || err != nil {
		return "", err
	}
	// variant bits; see section 4.1.1
	uuid[8] = uuid[8]&^0xc0 | 0x80
	// version 4 (pseudo-random); see section 4.1.3
	uuid[6] = uuid[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:]), nil
}
