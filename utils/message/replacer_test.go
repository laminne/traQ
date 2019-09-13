package message

import (
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestReplacer_Replace(t *testing.T) {
	t.Parallel()

	re := &Replacer{
		ChannelMap: map[string]uuid.UUID{
			"a": uuid.Must(uuid.FromString("ea452867-553b-4808-a14f-a47ee0009ee6")),
		},
		UserMap: map[string]uuid.UUID{
			"takashi_trap": uuid.Must(uuid.FromString("dfdff0c9-5de0-46ee-9721-2525e8bb3d45")),
		},
		GroupMap: map[string]uuid.UUID{},
	}

	tt := [][]string{
		{
			"aaaa#aeee `#a` @takashi_trapa @takashi_trap @#a\n```\n#a @takashi_trap\n```\n",
			"aaaa#aeee `#a` @takashi_trapa !{\"type\":\"user\",\"raw\":\"@takashi_trap\",\"id\":\"dfdff0c9-5de0-46ee-9721-2525e8bb3d45\"} @!{\"type\":\"channel\",\"raw\":\"#a\",\"id\":\"ea452867-553b-4808-a14f-a47ee0009ee6\"}\n```\n#a @takashi_trap\n```\n",
		},
	}
	for _, v := range tt {
		assert.Equal(t, v[1], re.Replace(v[0]))
	}
}
