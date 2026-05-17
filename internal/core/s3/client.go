// UploadObject 上传对象（仅用于同步器）
func (c *Client) UploadObject(ctx context.Context, key string, body io.Reader, contentType string) error {
	// 如果key已经包含listen_dir前缀，则不再重复添加
	fullKey := key
	if c.listenDir != "" && !strings.HasPrefix(key, c.listenDir) {
		fullKey = c.listenDir + key
	}

	input := &s3.PutObjectInput{
		Bucket:      &c.bucketName,
		Key:         &fullKey,
		Body:        body,
		ContentType: &contentType,
	}

	_, err := c.client.PutObject(ctx, input)
	if err != nil {
		var apiErr smithy.APIError
		if ok := errors.As(err, &apiErr); ok {
			util.Error("S3 API error", zap.String("code", apiErr.ErrorCode()), zap.String("message", apiErr.ErrorMessage()))
		}
		return err
	}

	return nil
}