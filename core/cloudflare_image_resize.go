package core

// Hosted Images optimization protocol names. These are provider grammar,
// not application breakpoints or transfer budgets.
// https://developers.cloudflare.com/images/optimization/features/
const (
	CloudflareImageWidthOption       = "width="
	CloudflareImageHeightOption      = "height="
	CloudflareImageFitOption         = "fit="
	CloudflareImageFormatOption      = "format="
	CloudflareImageMetadataOption    = "metadata="
	CloudflareImageAnimationOff      = "anim=false"
	CloudflareImageFitScaleDown      = "scale-down"
	CloudflareImageFitContain        = "contain"
	CloudflareImageFitCover          = "cover"
	CloudflareImageFitCrop           = "crop"
	CloudflareImageFitPad            = "pad"
	CloudflareImageFormatAuto        = "auto"
	CloudflareImageFormatAVIF        = "avif"
	CloudflareImageFormatWebP        = "webp"
	CloudflareImageFormatJSON        = "json"
	CloudflareImageMetadataNone      = "none"
	CloudflareImageMetadataCopyright = "copyright"
	CloudflareImageMetadataKeep      = "keep"
	CloudflareImageMediaTypePrefix   = "image/"
)
