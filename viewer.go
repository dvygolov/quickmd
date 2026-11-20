//go:build windows

package main

import (
	"image"
	"image/draw"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	viewerProcPtr uintptr
	viewerHwnd    uintptr
	viewerPix     []byte
	viewerW       int
	viewerH       int
	viewerInfo    BITMAPINFOHEADER
	viewerBrush   uintptr
	viewerClass   *uint16
	viewerTitle   *uint16
)

func viewerWndProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch msg {
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		viewerPaint(hwnd)
		return 0
	case WM_KEYDOWN:
		if wp == VK_ESCAPE {
			pDestroyWindow.Call(hwnd)
		}
		return 0
	case WM_LBUTTONDOWN:
		pDestroyWindow.Call(hwnd)
		return 0
	case WM_CLOSE:
		pDestroyWindow.Call(hwnd)
		return 0
	case WM_DESTROY:
		viewerHwnd = 0
		viewerPix = nil
		if hwndMain != 0 {
			pSetForegroundWindow.Call(hwndMain)
			pSetFocus.Call(hCode)
		}
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
}

// fitSize scales an image to fit a box without distortion.
func fitSize(boxW, boxH, imgW, imgH int) (int, int) {
	if imgW <= 0 || imgH <= 0 {
		return 0, 0
	}
	scale := float64(boxW) / float64(imgW)
	if s := float64(boxH) / float64(imgH); s < scale {
		scale = s
	}
	if scale > 1 {
		scale = 1
	}
	dw := int(float64(imgW) * scale)
	dh := int(float64(imgH) * scale)
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	return dw, dh
}

func viewerPaint(hwnd uintptr) {
	var ps PAINTSTRUCT
	hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	pPatBlt.Call(hdc, 0, 0, uintptr(viewerW), uintptr(viewerH), BLACKNESS)
	if len(viewerPix) > 0 {
		dw, dh := fitSize(viewerW, viewerH, int(viewerInfo.BiWidth), int(-viewerInfo.BiHeight))
		x := (viewerW - dw) / 2
		y := (viewerH - dh) / 2
		pSetStretchBltMode.Call(hdc, HALFTONE)
		pStretchDIBits.Call(hdc,
			uintptr(x), uintptr(y), uintptr(dw), uintptr(dh),
			0, 0, uintptr(int(viewerInfo.BiWidth)), uintptr(int(-viewerInfo.BiHeight)),
			uintptr(unsafe.Pointer(&viewerPix[0])),
			uintptr(unsafe.Pointer(&viewerInfo)),
			0, SRCCOPY)
	}
	if uiFont != 0 {
		pSetBkMode.Call(hdc, TRANSPARENT)
		pSetTextColor.Call(hdc, rgb(150, 150, 155))
		sendMsgFont(hdc, uiFont)
		rc := RECT{Left: 0, Top: 0, Right: int32(viewerW), Bottom: int32(viewerH) - 12}
		text := utf16Ptr("Esc — close")
		pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(text)), ^uintptr(0), uintptr(unsafe.Pointer(&rc)),
			DT_CENTER|DT_BOTTOM|DT_SINGLELINE|DT_NOPREFIX)
		runtime.KeepAlive(text)
	}
	pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
}

func sendMsgFont(hdc, font uintptr) {
	pSelectObject.Call(hdc, font)
}

func loadImageForViewer(path string) ([]byte, int, int, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, false
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, 0, 0, false
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, 0, 0, false
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(dst.Pix); i += 4 {
		dst.Pix[i+3] = 255
	}
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Over)
	pix := make([]byte, w*h*4)
	for i := 0; i < len(pix); i += 4 {
		pix[i+0] = dst.Pix[i+2]
		pix[i+1] = dst.Pix[i+1]
		pix[i+2] = dst.Pix[i+0]
		pix[i+3] = 255
	}
	return pix, w, h, true
}

func showImageViewer(path string) {
	pix, w, h, ok := loadImageForViewer(path)
	if !ok {
		messageBox("Could not open image:\n" + path)
		return
	}
	if viewerHwnd != 0 {
		pDestroyWindow.Call(viewerHwnd)
		viewerHwnd = 0
	}
	mon, _, _ := pMonitorFromWindow.Call(hwndMain, MONITOR_DEFAULTTONEAREST)
	var mi MONITORINFO
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	pGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi)))
	rc := mi.RcMonitor
	viewerW = int(rc.Right - rc.Left)
	viewerH = int(rc.Bottom - rc.Top)
	viewerPix = pix
	viewerInfo = BITMAPINFOHEADER{
		BiSize:        uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
		BiWidth:       int32(w),
		BiHeight:      -int32(h),
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: 0,
		BiSizeImage:   uint32(len(pix)),
	}
	if viewerProcPtr == 0 {
		viewerProcPtr = syscall.NewCallback(viewerWndProc)
		viewerClass = utf16Ptr("QuickMDViewer")
		viewerTitle = utf16Ptr("QuickMD")
		if viewerBrush == 0 {
			viewerBrush, _, _ = pCreateSolidBrush.Call(0)
		}
		var wc WNDCLASSEX
		wc.CbSize = uint32(unsafe.Sizeof(WNDCLASSEX{}))
		wc.LpfnWndProc = viewerProcPtr
		wc.HInstance = hInstance
		wc.HbrBackground = viewerBrush
		wc.LpszClassName = viewerClass
		pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
		runtime.KeepAlive(viewerClass)
	}
	hwnd, _, _ := pCreateWindowExW.Call(WS_EX_TOPMOST, uintptr(unsafe.Pointer(viewerClass)),
		uintptr(unsafe.Pointer(viewerTitle)), uintptr(WS_POPUP|WS_VISIBLE),
		uintptr(rc.Left), uintptr(rc.Top), uintptr(viewerW), uintptr(viewerH),
		0, 0, hInstance, 0)
	runtime.KeepAlive(viewerTitle)
	if hwnd == 0 {
		messageBox("Could not open the image viewer.")
		return
	}
	viewerHwnd = hwnd
	pSetForegroundWindow.Call(hwnd)
	pSetFocus.Call(hwnd)
	pInvalidateRect.Call(hwnd, 0, 1)
}

// openPreviewImage opens the image under the caret in the preview, if any.
func openPreviewImage() bool {
	if len(previewImages) == 0 {
		return false
	}
	var cr CHARRANGE
	sendMsg(hPreview, EM_EXGETSEL, 0, uintptr(unsafe.Pointer(&cr)))
	pos := int(cr.CpMin)
	for _, p := range []int{pos, pos - 1} {
		if path, ok := previewImages[p]; ok && path != "" {
			showImageViewer(path)
			return true
		}
	}
	return false
}
