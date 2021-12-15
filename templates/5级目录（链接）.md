.action{$docid:=.id}
.action{$block:= (queryBlocks "SELECT * FROM blocks WHERE type= 'd' AND path like '%/?/______________-_______.sy' Order BY hpath" $docid )}
.action{range $v:=$block} 
- [.action{$v.Content}](siyuan://block/.action{$v.ID})


    .action{$block:= (queryBlocks "SELECT * FROM blocks WHERE type= 'd' AND path like '%/?/______________-_______.sy' Order BY hpath" $v.ID)}
    .action{range $v:=$block}
    - [.action{$v.Content}](siyuan://block/.action{$v.ID})
      .action{$block:= (queryBlocks "SELECT * FROM blocks WHERE type= 'd' AND path like '%/?/______________-_______.sy' Order BY hpath" $v.ID)}
      .action{range $v:=$block}
      - [.action{$v.Content}](siyuan://block/.action{$v.ID})  
        .action{$block:= (queryBlocks "SELECT * FROM blocks WHERE type= 'd' AND path like '%/?/______________-_______.sy' Order BY hpath" $v.ID)}
        .action{range $v:=$block}
        - [.action{$v.Content}](siyuan://block/.action{$v.ID})
            .action{$block:= (queryBlocks "SELECT * FROM blocks WHERE type= 'd' AND path like '%/?/______________-_______.sy' Order BY hpath" $v.ID)}
            .action{range $v:=$block}
            - [.action{$v.Content}](siyuan://block/.action{$v.ID})  
            .action{end}
        .action{end}

     .action{end}                
            
    .action{end}
              
.action{end}


