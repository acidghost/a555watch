# a555watch

A 555 modern `watch` replacement.

<!--[[[cog
import cog
import subprocess
out = subprocess.run(
    ["./a555watch", "--help"],
    cwd="build",
    check=True,
    capture_output=True,
    text=True,
)
lines = list(out.stdout.splitlines())
lines = lines[1:-1]
help = "\n".join(lines)
cog.out(f"```\n{help}\n```")
]]]-->
```
    ┌───────────────────────────────────────────┐                      
    │                                           │                      
    │                                           │                      
    │                                           │                      
    │                             .      .      │                      
    │   ,-. .-- .-- .-- . , , ,-. |- ,-. |-.    │                      
    │   ,-| `-. `-. `-. |/|/  ,-| |  |   | |    │                      
    │   `-^ `-' `-' `-' ' '   `-^ `' `-' ' '    │                      
    │                                           │                      
    │                                           │                      
    │                                           │                      
    │                                           │                      
    └───────────────────────────────────────────┘                      
                                                                       
 ./a555watch [options] command                                         
                                                                       
   -n, --interval duration   time to wait between updates (default 2s) 
   -e, --errexit             exit if command has a non-zero exit       
   -g, --chgexit             exit when the output of command changes   
       --no-tui              do not use the TUI                        
       --no-alt              do not start the TUI in alt screen        
       --log string          write debug logs to file                  
       --debug               enable tracing logs                       
   -h, --help                display this help and exit                
```
<!--[[[end]]]-->
