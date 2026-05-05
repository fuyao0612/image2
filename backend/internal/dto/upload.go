package dto

// UploadImageResponse 表示图片上传成功后的返回体。
type UploadImageResponse struct {
	ImageURL    string `json:"imageUrl"`
	FileName    string `json:"fileName,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Size        int64  `json:"size,omitempty"`
}
