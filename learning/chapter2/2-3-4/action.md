## 操作流程
这里主要是按照书上的步骤创建了一个简易的 AUFS 文件系统, 注意不能在较新的内核中测试, 因为无法加载 AUFS 模块, 本次测试结果来源于 Ubuntu18.04-4.15.0-156-generic

具体操作步骤：
1. 首先加载 AUFS 模块：
    ```
    sudo modprobe aufs
    lsmod | grep aufs  # 检查是否加载成功
    ```
2. 然后以 AUFS 的方式进行挂载
    ```
    # 以 aufs 类型的文件系统将 dirs 所选择的目录挂载到 mnt 目录下
    sudo mount -t aufs -o dirs=./container-layer:./image-layer4:./image-layer3:./image-layer2:./image-layer1 none ./mnt
    ```
3. 查看挂载目录的读写情况
    ```
    # 只有 container-layer 目录是 read-write, 其他目录均为 read-only, 这也指明了挂载时,  
    # 会以 dirs 左侧第一个目录为 read-write, 其他均为 read-only
    root@frank:/home/frank/zdocker/learning/chapter2/2-3-4# cat /sys/fs/aufs/si_811df8ca80f91d52/*
    /home/frank/zdocker/learning/chapter2/2-3-4/container-layer=rw
    /home/frank/zdocker/learning/chapter2/2-3-4/image-layer4=ro
    /home/frank/zdocker/learning/chapter2/2-3-4/image-layer3=ro
    /home/frank/zdocker/learning/chapter2/2-3-4/image-layer2=ro
    /home/frank/zdocker/learning/chapter2/2-3-4/image-layer1=ro
    64
    65
    66
    67
    68
    home/frank/zdocker/learning/chapter2/2-3-4/container-layer/.aufs.xino
    ```
4. 最终挂载结果如 mnt 目录下所示
    ```
    mnt
    |-- container-layer.txt
    |-- image-layer1.txt
    |-- image-layer2.txt
    |-- image-layer3.txt
    `-- image-layer4.txt
    0 directories, 5 files
    ```
5. 接着执行实验操作
    ```
    echo -e "\nwrite to mnt's image-layer1.txt" >> ./mnt/image-layer4.txt
    ```
6. 查看结果
    ```
    root@frank:/home/frank/zdocker/learning/chapter2/2-3-4# cat ./mnt/image-layer4.txt 
    I am image layer 4
    write to mnt's image-layer1.txt
    ```
   从这里我们可以看到的确被修改了, 但 mnt 只是一个虚拟挂载点, 我们可以去到原目录查看
    ```
    root@frank:/home/frank/zdocker/learning/chapter2/2-3-4# cat image-layer4/image-layer4.txt 
    I am image layer 4
    ```
    可以看到原目录仍然是原样. 此时结合我们之前学到的 CoW, 以及 container-layer 目录作为 read-write 目录, 
    我们大致可以猜测变更在该目录：
    ```
    root@frank:/home/frank/zdocker/learning/chapter2/2-3-4# ls container-layer/
    container-layer.txt  image-layer4.txt
    root@frank:/home/frank/zdocker/learning/chapter2/2-3-4# cat container-layer/image-layer4.txt
    I am image layer 4
    write to mnt's image-layer1.txt
    ```
    首先可以发现的是该目录下多了一个 image-layer4.txt 文件, 并且文件内容也被修改了, 那这里就是我们说到的写时复制,
    在写入时将原文件拷贝到该目录下, 后续的写入操作均基于它
   