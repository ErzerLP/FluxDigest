package article

type SourceArticle struct {
	ID              string
	MinifluxEntryID int64
	FeedID          int64
	FeedTitle       string
	Title           string
	Author          string
	URL             string
	ContentHTML     string
	ContentText     string
	Fingerprint     string
}
