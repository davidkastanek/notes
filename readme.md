# Notes
Console app for taking quick notes in Markdown format
## Features
- Works with standard directory/files structure
- Shows navigation tree
- Shows notes preview
### Actions for directories
- New
  - Create new file specifying path ending with anything but slash
  - Create new dir specifying path ending with slash (`/`)
- Move - Change dir location
- Rename - Change dir name
- Delete - Delete dir
- Quit - Exit program
### Actions for files
- Edit - Open vim to edit the file
- Move - Change file location
- Rename - Change file name
- Delete - Delete file
- Quit - Exit program
## Usage
```
go build -o n
mv ./n ~/
~/n -d ~/Documents/notes
```
### Backups
Place the directory to any cloud file storage like Dropbox, Google Drive, etc. to keep your notes save and be able to get historical data if needed.
