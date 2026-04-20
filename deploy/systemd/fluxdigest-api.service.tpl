[Unit]
Description=FluxDigest API service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=__APP_USER__
Group=__APP_GROUP__
WorkingDirectory=__APP_DIR__
EnvironmentFile=__ENV_FILE__
Environment=APP_STATIC_DIR=__APP_DIR__/web/dist
ExecStart=__APP_DIR__/bin/rss-api
Restart=always
RestartSec=5s
TimeoutStopSec=20s
KillMode=process
NoNewPrivileges=true
LimitNOFILE=65535
SyslogIdentifier=fluxdigest-api

[Install]
WantedBy=multi-user.target
