package file

import (
	"bytes"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/traPtitech/traQ/model"
	"github.com/traPtitech/traQ/utils/imaging"
	"image/png"
)

// GenerateIconFile アイコンファイルを生成します
//
// 成功した場合、そのファイルのUUIDとnilを返します。
func GenerateIconFile(m Manager, salt string) (uuid.UUID, error) {
	var img bytes.Buffer
	icon := imaging.GenerateIcon(salt)

	if err := png.Encode(&img, icon); err != nil {
		return uuid.Nil, err
	}

	file, err := m.Save(SaveArgs{
		FileName:  fmt.Sprintf("%s.png", salt),
		FileSize:  int64(img.Len()),
		MimeType:  "image/png",
		FileType:  model.FileTypeIcon,
		Src:       &img,
		Thumbnail: icon,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return file.GetID(), nil
}