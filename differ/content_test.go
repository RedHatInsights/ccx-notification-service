package differ

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"
	utypes "github.com/RedHatInsights/insights-results-types"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func makeFetchRequest(testFileName string) (rules types.RulesMap, err error) {
	config, err := conf.LoadConfiguration(configFileEnvVariableName, defaultConfigFileName)
	if err != nil {
		return rules, err
	}
	server := httptest.NewServer(http.HandlerFunc(Handler))
	depencenciesConfiguration := conf.GetDependenciesConfiguration(&config)
	depencenciesConfiguration.ContentServiceServer = server.URL
	depencenciesConfiguration.ContentServiceEndpoint = "/tests/content/" + testFileName
	rules, err = fetchAllRulesContent(&depencenciesConfiguration)
	return rules, err
}

// TestFetchAllRulesContentOk cheks if the functon fetchAllRulesContent return valid  RulesMap
func TestFetchAllRulesContentOk(t *testing.T) {
	rules, err := makeFetchRequest("ok.json")
	assert.NoError(t, err)
	allContentFromMap := getAllContentFromMap(rules)
	assert.NotEmpty(t, allContentFromMap)
	assert.Len(t, allContentFromMap, 2)
}
func TestFetchAllRulesContentNoInternal(t *testing.T) {
	rules, err := makeFetchRequest("no_internal.json")
	assert.NoError(t, err)
	allContentFromMap := getAllContentFromMap(rules)
	assert.NotEmpty(t, allContentFromMap)
	assert.Len(t, allContentFromMap, 1)
}
func TestFetchAllRulesContentNoExternal(t *testing.T) {
	rules, err := makeFetchRequest("no_external.json")
	assert.NoError(t, err)
	allContentFromMap := getAllContentFromMap(rules)
	assert.Empty(t, allContentFromMap)
}
func TestFetchAllRulesContentMissing(t *testing.T) {
	rules, err := makeFetchRequest("missing.json")
	assert.NoError(t, err)
	allContentFromMap := getAllContentFromMap(rules)
	assert.Empty(t, allContentFromMap)
}
func Handler(w http.ResponseWriter, r *http.Request) {
	var con utypes.RuleContentDirectory
	print(r.RequestURI)
	jsonFile, err := os.Open(".." + r.RequestURI)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	jsonValue, err := io.ReadAll(jsonFile)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	err = json.Unmarshal(jsonValue, &con)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err = enc.Encode(con)
	var tosend string
	if err != nil {
		return
	}
	tosend = buffer.String()
	_, err = fmt.Fprint(w, tosend)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
}
