package main

import "testing"

func TestFetcherForRepoURL(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		want     repositoryFetcher
		wantRepo string
	}{
		{
			name:     "standard https git uses go-git",
			repoURL:  "https://github.com/example/infra.git",
			want:     repositoryFetcherGoGit,
			wantRepo: "https://github.com/example/infra.git",
		},
		{
			name:     "bgit plus https strips forcing prefix",
			repoURL:  "bgit+https://broker.example.com/demo.git",
			want:     repositoryFetcherBgit,
			wantRepo: "https://broker.example.com/demo.git",
		},
		{
			name:     "bgit plus file strips forcing prefix",
			repoURL:  "bgit+file://demo.git",
			want:     repositoryFetcherBgit,
			wantRepo: "file://demo.git",
		},
		{
			name:     "s3 local broker uses bgit",
			repoURL:  "s3://demo.git",
			want:     repositoryFetcherBgit,
			wantRepo: "s3://demo.git",
		},
		{
			name:     "gcs local broker uses bgit",
			repoURL:  "gs://demo.git",
			want:     repositoryFetcherBgit,
			wantRepo: "gs://demo.git",
		},
		{
			name:     "remote helper explicit url uses git helper",
			repoURL:  "bgit::https://broker.example.com/demo.git",
			want:     repositoryFetcherGitRemoteHelper,
			wantRepo: "bgit::https://broker.example.com/demo.git",
		},
		{
			name:     "remote helper shorthand uses git helper",
			repoURL:  "bgit://demo.git",
			want:     repositoryFetcherGitRemoteHelper,
			wantRepo: "bgit://demo.git",
		},
		{
			name:     "plain file url remains ordinary git",
			repoURL:  "file:///repos/demo.git",
			want:     repositoryFetcherGoGit,
			wantRepo: "file:///repos/demo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotRepo := fetcherForRepoURL(tt.repoURL)
			if got != tt.want {
				t.Fatalf("fetcherForRepoURL() fetcher = %v, want %v", got, tt.want)
			}
			if gotRepo != tt.wantRepo {
				t.Fatalf("fetcherForRepoURL() repo = %q, want %q", gotRepo, tt.wantRepo)
			}
		})
	}
}
