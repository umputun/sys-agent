package external

import (
	"net/url"
	"testing"
	"time"

	"github.com/go-pkgz/mongo/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestMongoProvider_Status(t *testing.T) {
	_, _, teardown := mongo.MakeTestConnection(t)
	defer teardown()

	{
		p := MongoProvider{TimeOut: time.Second}
		resp, err := p.Status(Request{Name: "test", URL: "mongodb://localhost:27017"})
		require.NoError(t, err)

		assert.Equal(t, "test", resp.Name)
		assert.Equal(t, 200, resp.StatusCode)
		assert.True(t, resp.ResponseTime > 0)
		assert.Equal(t, map[string]interface{}{"status": "ok"}, resp.Body)
	}

	{
		p := MongoProvider{TimeOut: time.Second}
		_, err := p.Status(Request{Name: "test", URL: "mongodb://localhost:27000"})
		require.Error(t, err)
	}
}

func TestMongoProvider_parseReplStatus(t *testing.T) {
	p := MongoProvider{TimeOut: time.Second}

	{
		uu, err := url.Parse("mongodb://localhost:27017?oplogMaxDelta=55s")
		require.NoError(t, err)

		data := bson.M{"set": "rs0", "members": primitive.A{
			bson.M{"name": "node1", "stateStr": "PRIMARY", "optime": bson.M{"ts": primitive.Timestamp{T: 1514908344, I: 123}}},
			bson.M{"name": "node2", "stateStr": "SECONDARY", "optime": bson.M{"ts": primitive.Timestamp{T: 1514908384, I: 456}}},
			bson.M{"name": "node3", "stateStr": "SECONDARY", "optime": time.Date(2018, time.January, 2, 15, 51, 33, 0, time.UTC)},
			bson.M{"name": "node4", "stateStr": "ARBITER"},
		}}

		res, err := p.parseReplStatus(uu, data)
		require.NoError(t, err)
		assert.Equal(t, "ok", res.Status)
		assert.Equal(t, "ok", res.OptimeStatus)
		assert.Equal(t, 4, len(res.Members))
		assert.Equal(t, "PRIMARY", res.Members[0].State)
		assert.Equal(t, "SECONDARY", res.Members[1].State)
		assert.Equal(t, "SECONDARY", res.Members[2].State)
		assert.Equal(t, "ARBITER", res.Members[3].State)
		t.Logf("%+v", res)
	}

	{ // large optime difference
		uu, err := url.Parse("mongodb://localhost:27017?oplogMaxDelta=55s")
		require.NoError(t, err)

		data := bson.M{"set": "rs0", "members": primitive.A{
			bson.M{"name": "node1", "stateStr": "PRIMARY", "optime": bson.M{"ts": primitive.Timestamp{T: 1514908344, I: 123}}},
			bson.M{"name": "node2", "stateStr": "SECONDARY", "optime": bson.M{"ts": primitive.Timestamp{T: 1514905384, I: 456}}},
			bson.M{"name": "node3", "stateStr": "SECONDARY", "optime": time.Date(2018, time.January, 2, 15, 51, 33, 0, time.UTC)},
			bson.M{"name": "node4", "stateStr": "ARBITER"},
		}}

		res, err := p.parseReplStatus(uu, data)
		require.NoError(t, err)
		assert.Equal(t, "ok", res.Status)
		assert.Equal(t, "failed, optime difference for node2 is 49m19.999999667s", res.OptimeStatus)
		assert.Equal(t, 4, len(res.Members))
		t.Logf("%+v", res)
	}

	{ // invalid oplogMaxDelta
		uu, err := url.Parse("mongodb://localhost:27017?oplogMaxDelta=55xx")
		require.NoError(t, err)

		data := bson.M{"set": "rs0", "members": primitive.A{
			bson.M{"name": "node1", "stateStr": "PRIMARY", "optime": bson.M{"ts": primitive.Timestamp{T: 1514908344, I: 123}}},
			bson.M{"name": "node2", "stateStr": "SECONDARY", "optime": bson.M{"ts": primitive.Timestamp{T: 1514908384, I: 456}}},
			bson.M{"name": "node3", "stateStr": "SECONDARY", "optime": time.Date(2018, time.January, 2, 15, 51, 33, 0, time.UTC)},
			bson.M{"name": "node4", "stateStr": "ARBITER"},
		}}

		_, err = p.parseReplStatus(uu, data)
		require.EqualError(t, err, `can't parse oplogMaxDelta: localhost:27017: time: unknown unit "xx" in duration "55xx"`)
	}

	{ // no members
		uu, err := url.Parse("mongodb://localhost:27017?oplogMaxDelta=55s")
		require.NoError(t, err)

		data := bson.M{"set": "rs0"}
		_, err = p.parseReplStatus(uu, data)
		require.EqualError(t, err, "mongo replset members can't be extracted: <nil>")

		data = bson.M{"set": "rs0", "members": primitive.A{}}
		_, err = p.parseReplStatus(uu, data)
		require.EqualError(t, err, "mongo replset is empty")
	}

	{ // no set
		uu, err := url.Parse("mongodb://localhost:27017?oplogMaxDelta=55s")
		require.NoError(t, err)

		data := bson.M{"members": primitive.A{
			bson.M{"name": "node1", "stateStr": "PRIMARY", "optime": bson.M{"ts": primitive.Timestamp{T: 1514908344, I: 123}}},
			bson.M{"name": "node2", "stateStr": "SECONDARY", "optime": bson.M{"ts": primitive.Timestamp{T: 1514908384, I: 456}}},
			bson.M{"name": "node3", "stateStr": "SECONDARY", "optime": time.Date(2018, time.January, 2, 15, 51, 33, 0, time.UTC)},
			bson.M{"name": "node4", "stateStr": "ARBITER"},
		}}

		_, err = p.parseReplStatus(uu, data)
		assert.EqualError(t, err, `mongo replset set can't be extracted: <nil>`)
	}

	{ // members wrong type
		uu, err := url.Parse("mongodb://localhost:27017?oplogMaxDelta=55s")
		require.NoError(t, err)

		data := bson.M{"set": "rs0", "members": 123}

		_, err = p.parseReplStatus(uu, data)
		require.EqualError(t, err, `mongo replset members can't be extracted: 123`)
	}

	{ // members status
		uu, err := url.Parse("mongodb://localhost:27017?oplogMaxDelta=55s")
		require.NoError(t, err)

		data := bson.M{"set": "rs0", "members": primitive.A{
			bson.M{"name": "node1", "stateStr": 1234, "optime": bson.M{"ts": primitive.Timestamp{T: 1514908344, I: 123}}},
			bson.M{"name": "node2", "stateStr": "SECONDARY", "optime": bson.M{"ts": primitive.Timestamp{T: 1514908384, I: 456}}},
			bson.M{"name": "node3", "stateStr": "SECONDARY", "optime": time.Date(2018, time.January, 2, 15, 51, 33, 0, time.UTC)},
			bson.M{"name": "node4", "stateStr": "ARBITER"},
		}}

		_, err = p.parseReplStatus(uu, data)
		require.EqualError(t, err, `mongo replset member state can't be extracted: map[name:node1 optime:map[ts:{T:1514908344 I:123}] stateStr:1234]`)
	}

	{ // members status
		uu, err := url.Parse("mongodb://localhost:27017?oplogMaxDelta=55s")
		require.NoError(t, err)

		data := bson.M{}
		_, err = p.parseReplStatus(uu, data)
		require.EqualError(t, err, `mongo replset members can't be extracted: <nil>`)
	}
}
