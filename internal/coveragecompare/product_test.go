//go:build test

package coveragecompare

import "testing"

func TestIsGoProduct(t *testing.T) {
	tests := []struct {
		name string
		file string
		src  string
		want bool
	}{
		{
			name: "file with no build line",
			file: "internal/x/x.go",
			src:  "package x\n",
			want: true,
		},
		{
			name: "test tag",
			file: "internal/x/x.go",
			src:  "//go:build test\n\npackage x\n",
			want: false,
		},
		{
			name: "test or integration",
			file: "internal/x/x.go",
			src:  "//go:build test || integration\n\npackage x\n",
			want: false,
		},
		{
			name: "test and not race",
			file: "internal/x/x.go",
			src:  "//go:build test && !race\n\npackage x\n",
			want: false,
		},
		{
			name: "test and race",
			file: "internal/x/x.go",
			src:  "//go:build test && race\n\npackage x\n",
			want: false,
		},
		{
			name: "test and unix",
			file: "internal/x/x.go",
			src:  "//go:build test && unix\n\npackage x\n",
			want: false,
		},
		{
			name: "a negated test tag stays product code",
			file: "internal/x/x.go",
			src:  "//go:build !test\n\npackage x\n",
			want: true,
		},
		{
			name: "a negated test tag with another tag stays product code",
			file: "internal/x/x.go",
			src:  "//go:build !test && unix\n\npackage x\n",
			want: true,
		},
		{
			name: "a negated conjunction with test stays product code",
			file: "internal/x/x.go",
			src:  "//go:build !(test && race)\n\npackage x\n",
			want: true,
		},
		{
			name: "integration only stays product code",
			file: "internal/x/x.go",
			src:  "//go:build integration\n\npackage x\n",
			want: true,
		},
		{
			name: "unix stays product code",
			file: "internal/x/x.go",
			src:  "//go:build unix\n\npackage x\n",
			want: true,
		},
		{
			name: "test written second in the expression",
			file: "internal/x/x.go",
			src:  "//go:build integration || test\n\npackage x\n",
			want: false,
		},
		{
			name: "test file",
			file: "internal/x/x_test.go",
			src:  "package x\n",
			want: false,
		},
		{
			name: "build line inside a string after the package clause is not a constraint",
			file: "internal/x/x.go",
			src:  "package x\n\nvar s = `\n//go:build test\n`\n",
			want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := isGoProduct(test.file, []byte(test.src), PointBase)
			if err != nil {
				t.Fatalf("isGoProduct: %v", err)
			}
			if got != test.want {
				t.Errorf("isGoProduct = %v, want %v", got, test.want)
			}
		})
	}
}

func TestIsGoProductRejectsBrokenBuildLines(t *testing.T) {
	tests := map[string]string{
		"malformed expression": "//go:build test &&\n\npackage x\n",
		"two build lines":      "//go:build test\n//go:build unix\n\npackage x\n",
	}
	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := isGoProduct("internal/x/x.go", []byte(src), PointBase)
			requireStop(t, err, PointBase, InputSource)
		})
	}
}
