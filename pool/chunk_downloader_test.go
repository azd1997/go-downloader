package pool

import (
	"net/http"
	"testing"
)

func TestChunkDownloader_Download(t *testing.T) {
	
	tryagain := make(chan *Chunk, 10)
	cd := NewChunkDownloader(1, tryagain)
	
	chunk := &Chunk{
		Begin:   0,
		End:     10000,
		Url:     "",
		DbPath:  "",
		DataKey: "",
		TaskKey: "",
		tried:   0,
	}
	
	type fields struct {
		id        int
		status    Status
		client    *http.Client
		tryAgain  chan<- *Chunk
		cacheSize int
	}
	type args struct {
		chunk *Chunk
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cd := &ChunkDownloader{
				id:        tt.fields.id,
				status:    tt.fields.status,
				client:    tt.fields.client,
				tryAgain:  tt.fields.tryAgain,
				cacheSize: tt.fields.cacheSize,
			}
			if err := cd.Download(tt.args.chunk); (err != nil) != tt.wantErr {
				t.Errorf("Download() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
