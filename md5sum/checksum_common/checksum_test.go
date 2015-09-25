package checksum_common

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestCalc_checksum(t *testing.T) {
	cases := []struct {
		in, want, t string
	}{
		/* md5 */
		{"hello, world", "e4d7f1b4ed2e42d15898f4b27b019da4", "md5"},
		{"ad3344412123123fasdfasdf", "353a3336352ae74136ef5b37e4091c4c", "md5"},
		{"333dddf213sfasdfasdfasfd\n", "106f56f032f7d29af6af98eeb24d5d2c", "md5"},

		/* sha1 */
		{"hello, world", "b7e23ec29af22b0b4e41da31e868d57226121c84", "sha1"},
		{"ad3344412123123fasdfasdf", "76758123ef52b09899546142393c32db7f53a022", "sha1"},
		{"333dddf213sfasdfasdfasfd\n", "0d966748d89029d547f3e36e63af9839f3b9ce6b", "sha1"},

		/* sha224 */
		{"hello, world", "6e1a93e32fb44081a401f3db3ef2e6e108b7bbeeb5705afdaf01fb27", "sha224"},
		{"ad3344412123123fasdfasdf", "372b81303da6fd1418041af497d53b9434891b9c93954e929d4ee7a3", "sha224"},
		{"333dddf213sfasdfasdfasfd\n", "cc92bf4cc34f856bfedc056f943afc68121f74620ca158f42920765f", "sha224"},

		/* sh256 */
		{"hello, world", "09ca7e4eaa6e8ae9c7d261167129184883644d07dfba7cbfbc4c8a2e08360d5b", "sha256"},
		{"ad3344412123123fasdfasdf", "a1962c3391c5580497624394b3b156637ec0e33d6705938c505b4a20e7e73fce", "sha256"},
		{"333dddf213sfasdfasdfasfd\n", "19bacc61eb1896d9865d77f62c19a325b704a7a23ef43e2cb46d9b406c01ab28", "sha256"},

		/* sha384 */
		{"hello, world", "1fcdb6059ce05172a26bbe2a3ccc88ed5a8cd5fc53edfd9053304d429296a6da23b1cd9e5c9ed3bb34f00418a70cdb7e", "sha384"},
		{"ad3344412123123fasdfasdf", "494dd728eeccfa6cc27815538ab432c4dbb19d0f7157654fb3fc945fd2f6535535d4af8dbefbefbd51d608b4423c9508", "sha384"},
		{"333dddf213sfasdfasdfasfd\n", "8823b5ac0abaa00b665525e74072f233a6a8f0291cf6974d04ab548acbb11e860f13b156a787987cd9cd799306d4b138", "sha384"},

		/* sha512 */
		{"hello, world", "8710339dcb6814d0d9d2290ef422285c9322b7163951f9a0ca8f883d3305286f44139aa374848e4174f5aada663027e4548637b6d19894aec4fb6c46a139fbf9", "sha512"},
		{"ad3344412123123fasdfasdf", "e08719391e0e3592db97bf24084ea5230f645da3cb5747aa10e504feafc53426348a61ea9b392be255ac89c28a2ed9092d433b377292827a65a897a2a7687a07", "sha512"},
		{"333dddf213sfasdfasdfasfd\n", "c7fb59d56d18f86c6838a3a504dcc939e11eec832338f5ef998f222f7cd66527536499dcfac5b8649381adf7665e0557e3574febca7d605a0798b8f737d05f54", "sha512"},
	}

	for _, v := range cases {
		buf := bytes.NewBufferString(v.in)
		sum := calc_checksum(buf, v.t)
		if sum != v.want {
			t.Errorf("%ssum (%#v) == %#v, want %#v", v.t, v.in, sum, v.want)
		} else {
			t.Logf("%ssum (%#v), expect %#v, got %#v\n", v.t, v.in, v.want, sum)
		}
	}
}

func TestCheck_checksum(t *testing.T) {

	old_stdout := os.Stdout

	/* capture output */
	r, w, _ := os.Pipe()
	os.Stdout = w

	outC := make(chan string)

	defer func() {
		w.Close()
		os.Stdout = old_stdout
		output := <-outC
		t.Logf("%s\n", output)
	}()

	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	sum_methods := []string{"md5", "sha1", "sha224", "sha256", "sha384", "sha512"}
	for _, m := range sum_methods {
		fn := fmt.Sprintf("testdata/checksum.%s", m)
		sum_f_lists := []string{fn}
		if r := CompareChecksum(sum_f_lists, m, true, true); !r {
			t.Fail()
		} else {
			t.Logf("check %s for %s: success\n", fn, m)
		}
	}
}

func TestGenChecksum(t *testing.T) {

	old_stdout := os.Stdout

	/* capture output */
	r, w, _ := os.Pipe()
	os.Stdout = w

	outC := make(chan string)

	defer func() {
		w.Close()
		os.Stdout = old_stdout
		output := <-outC
		t.Logf("\n%s\n", output)
	}()

	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	sum_methods := []string{"md5", "sha1", "sha224", "sha256", "sha384", "sha512"}
	for _, m := range sum_methods {
		flists := []string{"testdata/*.txt"}
		if r := GenerateChecksum(flists, m); !r {
			t.Fail()
		} else {
			t.Logf("generate %sum: success\n", m)
		}
	}
}
