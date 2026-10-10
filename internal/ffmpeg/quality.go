package ffmpeg

import "strconv"

// QualityArgs 依据编码器映射质量参数（CRF 语义近似）。探测自检与正式任务共用，确保探测覆盖真实参数组合。
func QualityArgs(encoder string, crf int) []string {
	q := strconv.Itoa(crf)
	switch encoder {
	case "libx264":
		return []string{"-crf", q}
	case "h264_nvenc", "hevc_nvenc":
		return []string{"-rc", "vbr", "-cq", q}
	case "h264_qsv", "hevc_qsv":
		return []string{"-global_quality", q}
	case "h264_amf", "hevc_amf":
		return []string{"-qp_i", q, "-qp_p", q}
	default:
		return nil
	}
}
