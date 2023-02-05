This program opens two serial ports specified in the options and forwards data between them as if it is just a wire.
RTS & CTS Control flow will be forwarded (& optionally logged) as well.
The data will be written as hexdump to stdout and optionally to a file.

If you intend to use this on Windows have a look at the com0com project: https://com0com.sourceforge.net/


```Usage of serial-proxy:
  -h, --help                 Show this help dialog
      --left-label string    an arbitrary label for the left port, used for better distinction in the logs (default "Left Port")
  -l, --left-port string     Left port definition (default "COM1,19200,N,8,1")
      --log-control-flow     Log control flow (CTS / DTR)
  -o, --output string        log file. leave this emtpy to log to console only
      --read-bugger int      Read buffer size (default 4096)
  -t, --read-timeout int     Read timeout in ms. Adjust this to better detect packet boundaries (default 100)
      --right-label string   an arbitrary label for the left port, used for better distinction in the logs (default "Right Port")
  -r, --right-port string    Right port definition (default "COM2,19200,N,8,1")```