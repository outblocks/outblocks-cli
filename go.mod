module github.com/outblocks/outblocks-cli

go 1.16

require (
	github.com/Masterminds/vcs v1.13.1
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d // indirect
	github.com/blang/semver/v4 v4.0.0
	github.com/enescakir/emoji v1.0.0
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/go-ozzo/ozzo-validation/v4 v4.3.0
	github.com/go-playground/validator/v10 v10.5.0 // indirect
	github.com/goccy/go-yaml v1.8.9
	github.com/google/go-github/v35 v35.0.0
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/mholt/archiver/v3 v3.5.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/mapstructure v1.4.1 // indirect
	github.com/otiai10/copy v1.5.1
	github.com/outblocks/outblocks-plugin-go v0.0.0
	github.com/pelletier/go-toml v1.9.0 // indirect
	github.com/pterm/pterm v0.12.13
	github.com/spf13/afero v1.6.0 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	golang.org/x/oauth2 v0.0.0-20210413134643-5e61552d6c78
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	golang.org/x/sys v0.0.0-20210423082822-04245dca01da // indirect
	golang.org/x/text v0.3.6 // indirect
	gopkg.in/ini.v1 v1.62.0 // indirect
)

replace github.com/goccy/go-yaml => github.com/23doors/go-yaml v1.8.10-0.20210513211449-7c6c82dc3f03

replace github.com/outblocks/outblocks-plugin-go => ../outblocks-plugin-go
