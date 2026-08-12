package insights

import (
	"errors"
	"reflect"
	"testing"
)

func TestMiyunFilterCatalogNormalizesLabelsAliasesAndIDs(t *testing.T) {
	t.Parallel()
	mtypes, err := normalizeMiyunMTypes([]string{"视频", "202", "vertical_video", "视频"})
	if err != nil || !reflect.DeepEqual(mtypes, []string{"201", "202"}) {
		t.Fatalf("mtypes=%v err=%v", mtypes, err)
	}
	tags, err := normalizeMiyunMaterialTags([]string{"商品展示", "single_speaker", "5"})
	if err != nil || !reflect.DeepEqual(tags, []string{"1", "5"}) {
		t.Fatalf("materialTags=%v err=%v", tags, err)
	}
	if _, err := normalizeMiyunMaterialTags([]string{"测评"}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("unsupported materialTag err=%v", err)
	}
}
