---
page_title: "Filestation: synology_filestation_file"
subcategory: "Filestation"
description: |-
  A file on the Synology NAS Filestation.
---

# Filestation: File (Resource)

A file on the Synology NAS Filestation.



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `path` (String) A destination folder path starting with a shared folder to which files can be uploaded.

### Optional

- `content` (String) The raw file contents to add to the Synology NAS.
- `create_parents` (Boolean) Create parent folder(s) if none exist.
- `overwrite` (Boolean) Overwrite the destination file if one exists.
- `url` (String) A file url to download and add to the Synology NAS.

### Read-Only

- `access_time` (String) The time the file was last accessed.
- `change_time` (String) The time the file was last changed.
- `create_time` (String) The time the file was created.
- `md5` (String) The MD5 hash of the file.
- `modified_time` (String) The time the file was last modified.
- `real_path` (String) The real path of the folder.