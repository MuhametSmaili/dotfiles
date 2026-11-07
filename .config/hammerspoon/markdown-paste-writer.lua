function Writer(document, options)
	document = document:walk({
		RawBlock = function(block)
			return pandoc.CodeBlock(block.text)
		end,
		RawInline = function(inline)
			return pandoc.Str(inline.text)
		end,
		Image = function(image)
			return image.caption
		end,
		Link = function(link)
			local scheme = link.target:match("^([%a][%w+.-]*):")
			scheme = scheme and scheme:lower()
			if scheme ~= "http" and scheme ~= "https" and scheme ~= "mailto" then
				return link.content
			end
		end,
	})
	local html = pandoc.write(document, "html5", options)
	html = html:gsub("<pre([^>]*)>", '<pre%1 style="white-space:pre-wrap;font-family:monospace;">')
	html = html:gsub("<code>", '<code style="font-family:monospace;">')
	return pandoc.json.encode({
		html = '<html><head><meta charset="utf-8"></head><body>' .. html .. "</body></html>",
		text = pandoc.write(document, "plain", options),
	})
end
