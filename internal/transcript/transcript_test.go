package transcript

import "testing"

func TestFormats(t *testing.T) {
	cases := map[string]string{
		"bcc":     `{"font_size":0.4,"body":[{"from":0.5,"to":2.1,"content":"第一句"},{"from":2.3,"to":4,"content":"第二句"}]}`,
		"whisper": `{"segments":[{"start":0.5,"end":2.1,"text":" 第一句"},{"start":2.3,"end":4.0,"text":"第二句"}]}`,
		"vtt":     "WEBVTT\n\n00:00.500 --> 00:02.100 align:start\n<c>第一句</c>\n\n00:02.300 --> 00:04.000\n第二句\n",
		"srt":     "\ufeff1\r\n00:00:00,500 --> 00:00:02,100\r\n第一句\r\n\r\n2\r\n00:00:02,300 --> 00:00:04,000\r\n第二句\r\n",
	}
	for name, raw := range cases {
		cues, err := Parse([]byte(raw))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(cues) != 2 || cues[0].Text != "第一句" || cues[1].Start != 2.3 || cues[1].End != 4 {
			t.Errorf("%s: %+v", name, cues)
		}
	}
}

func TestClock(t *testing.T) {
	if Clock(153.4) != "2:33" || Clock(3725) != "1:02:05" {
		t.Fatal(Clock(153.4), Clock(3725))
	}
}
