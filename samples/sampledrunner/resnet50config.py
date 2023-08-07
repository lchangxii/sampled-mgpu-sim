def init_resnet50_101_152():
    benchmarks = dict()
    benchmarks["conv2d"] = []
    batch=1
    enable_backward=False
                    ## in,  w,  h, co,kwh,pwh,stridewh
    convparams=[                                                     ##group       inner innerinner
             [ batch,    3,224,224,  64,7,7,3,3,2,2,enable_backward ],  ## 1
             [ batch,   64, 56, 56,  64,1,1,0,0,1,1,enable_backward ],  ## 2,        1,   1
             [ batch,   64, 56, 56,  64,3,3,1,1,1,1,enable_backward ],  ## 2,        1,   2
             [ batch,   64, 56, 56, 256,1,1,0,0,1,1,enable_backward ],  ## 2,        1,   3/downsampling
             [ batch,  256, 56, 56,  64,1,1,0,0,1,1,enable_backward ],  ## 2,       2/3,   1
             [ batch,   64, 56, 56,  64,3,3,1,1,1,1,enable_backward ],  ## 2,       2/3,   2
             [ batch,   64, 56, 56, 256,1,1,0,0,1,1,enable_backward ],  ## 2,       2/3,   3
    
             [ batch,  256, 56, 56, 128,1,1,0,0,1,1,enable_backward ],  ## 3,        1,   1
             [ batch,  128, 56, 56, 128,3,3,1,1,2,2,enable_backward ],  ## 3,        1,   2
             [ batch,  128, 28, 28, 512,1,1,0,0,1,1,enable_backward ],  ## 3,        1,   3
             [ batch,  256, 56, 56, 512,1,1,0,0,2,2,enable_backward ],  ## 3,        1,   downsampling
             [ batch,  512, 28, 28, 128,1,1,0,0,1,1,enable_backward ],  ## 3,    2/3/4,   1
             [ batch,  128, 28, 28, 128,3,3,1,1,1,1,enable_backward ],  ## 3,    2/3/4,   2
             [ batch,  128, 28, 28, 512,1,1,0,0,1,1,enable_backward ],  ## 3,    2/3/4,   3
    
             [ batch,  512, 28, 28, 256,1,1,0,0,1,1,enable_backward ],  ## 4,        1,   1
             [ batch,  256, 28, 28, 256,3,3,1,1,2,2,enable_backward ],  ## 4,        1,   2
             [ batch,  256, 14, 14,1024,1,1,0,0,1,1,enable_backward ],  ## 4,        1,   3
             [ batch,  512, 28, 28,1024,1,1,0,0,2,2,enable_backward ],  ## 4,        1,   downsampling
             [ batch, 1024, 14, 14, 256,1,1,0,0,1,1,enable_backward ],  ## 4,2/3/4/5/6,   1
             [ batch,  256, 14, 14, 256,3,3,1,1,1,1,enable_backward ],  ## 4,2/3/4/5/6,   2
             [ batch,  256, 14, 14,1024,1,1,0,0,1,1,enable_backward ],  ## 4,2/3/4/5/6,   3
    
             [ batch, 1024, 14, 14, 512,1,1,0,0,1,1,enable_backward ],  ## 5,        1,   1
             [ batch,  512,  7,  7, 512,3,3,1,1,2,2,enable_backward ],  ## 5,        1,   2
             [ batch,  512,  7,  7,2048,1,1,0,0,1,1,enable_backward ],  ## 5,        1,   3
             [ batch, 1024, 14, 14,2048,1,1,0,0,2,2,enable_backward ],  ## 5,        1,   downsampling
             [ batch, 2048,  7,  7, 512,1,1,0,0,1,1,enable_backward ],  ## 5,      2/3,   1
             [ batch,  512,  7,  7, 512,3,3,1,1,1,1,enable_backward ],  ## 5,      2/3,   2
             [ batch,  512,  7,  7,2048,1,1,0,0,1,1,enable_backward ],  ## 5,      2/3,   3
            ]
    for param in convparams:
        cmd = "./conv2d -N %d -C %d -H %d -W %d -output-channel %d -kernel-height %d -kernel-width %d -pad-x %d -pad-y %d -stride-x %d -stride-y %d"%(param[0],param[1],param[2],param[3],param[4],param[5],param[6],param[7],param[8],param[9],param[10])
        if param[11]:
            cmd += " -enable-backward"
        benchmarks["conv2d"].append( cmd )
    benchmarks["maxpooling"] = []
    params=[                                                           ##group inner
             [ batch,  64,112,112, 64 ,3,3,1,1,2,2,enable_backward ],  ## 1
            ]
    for param in params:
        cmd = "./maxpooling -N %d -C %d -H %d -W %d -output-channel %d -kernel-height %d -kernel-width %d -pad-x %d -pad-y %d -stride-x %d -stride-y %d"%(param[0],param[1],param[2],param[3],param[4],param[5],param[6],param[7],param[8],param[9],param[10])
        if param[11]:
            cmd += " -enable-backward"
        benchmarks["maxpooling"].append( cmd )
    
    benchmarks["avgpooling"] = []
    params=[                                                           ##group inner
             [ batch, 2048,7,7, 2048 ,7,7,0,0,1,1,enable_backward ],  ## 6
            ]
    for param in params:
        cmd = "./avgpooling -N %d -C %d -H %d -W %d -output-channel %d -kernel-height %d -kernel-width %d -pad-x %d -pad-y %d -stride-x %d -stride-y %d"%(param[0],param[1],param[2],param[3],param[4],param[5],param[6],param[7],param[8],param[9],param[10])
        if param[11]:
            cmd += " -enable-backward"
        benchmarks["avgpooling"].append( cmd )
    
    
    
    benchmarks["fulllayer"] = []
    params=[                                               ##group inner
             [ batch,  2048, 1000,enable_backward ],       ## 7,   0
            ]
    for param in params:
        cmd = "./fulllayer -N %d -input-dim %d -output-dim %d "%(param[0],param[1],param[2])
        if param[-1]:
            cmd += " -enable-backward"
        benchmarks["fulllayer"].append( cmd )

    return benchmarks
def run_resnet(config,benchmarks): ##(2,)
    benchparams = [] 
    conv2ds = benchmarks["conv2d"]
    conv2d_1_1_1 = conv2ds[0]
    conv2d_2_1_1 = conv2ds[1]
    conv2d_2_1_2 = conv2ds[2]
    conv2d_2_1_3 = conv2ds[3]
    conv2d_2_1_downsample = conv2ds[3]
    conv2d_2_2_1 = conv2ds[4]
    conv2d_2_2_2 = conv2ds[5]
    conv2d_2_2_3 = conv2ds[6]
####################################
    conv2d_3_1_1 = conv2ds[7]
    conv2d_3_1_2 = conv2ds[8]
    conv2d_3_1_3 = conv2ds[9]
    conv2d_3_1_downsample = conv2ds[10]
    conv2d_3_2_1 = conv2ds[11]
    conv2d_3_2_2 = conv2ds[12]
    conv2d_3_2_3 = conv2ds[13]
################################
    conv2d_4_1_1 = conv2ds[14]
    conv2d_4_1_2 = conv2ds[15]
    conv2d_4_1_3 = conv2ds[16]
    conv2d_4_1_downsample = conv2ds[17]
    conv2d_4_2_1 = conv2ds[18]
    conv2d_4_2_2 = conv2ds[19]
    conv2d_4_2_3 = conv2ds[20]
################################
    conv2d_5_1_1 = conv2ds[21]
    conv2d_5_1_2 = conv2ds[22]
    conv2d_5_1_3 = conv2ds[23]
    conv2d_5_1_downsample = conv2ds[24]
    conv2d_5_2_1 = conv2ds[25]
    conv2d_5_2_2 = conv2ds[26]
    conv2d_5_2_3 = conv2ds[27]

    avgpoolings = benchmarks[ "avgpooling" ]
    avgpooling_0 = avgpoolings[0]
    maxpoolings = benchmarks[ "maxpooling" ]
    maxpooling_0 = maxpoolings[0]

    benchparams.append( ("conv2d", conv2d_1_1_1) )
    benchparams.append( ("maxpooling", maxpooling_0) )

    benchparams.append( ("conv2d", conv2d_2_1_1) )
    benchparams.append( ("conv2d", conv2d_2_1_2) )
    benchparams.append( ("conv2d", conv2d_2_1_3) )
    benchparams.append( ("conv2d", conv2d_2_1_downsample) )
    for i in range(config[0]):
        benchparams.append( ("conv2d", conv2d_2_2_1) )
        benchparams.append( ("conv2d", conv2d_2_2_2) )
        benchparams.append( ("conv2d", conv2d_2_2_3) )

    benchparams.append( ("conv2d", conv2d_3_1_1) )
    benchparams.append( ("conv2d", conv2d_3_1_2) )
    benchparams.append( ("conv2d", conv2d_3_1_3) )
    benchparams.append( ("conv2d", conv2d_3_1_downsample) )
    for i in range(config[1]):
        benchparams.append( ("conv2d", conv2d_3_2_1) )
        benchparams.append( ("conv2d", conv2d_3_2_2) )
        benchparams.append( ("conv2d", conv2d_3_2_3) )

    benchparams.append( ("conv2d", conv2d_4_1_1) )
    benchparams.append( ("conv2d", conv2d_4_1_2) )
    benchparams.append( ("conv2d", conv2d_4_1_3) )
    benchparams.append( ("conv2d", conv2d_4_1_downsample) )
    for i in range(config[2]):
        benchparams.append( ("conv2d", conv2d_4_2_1) )
        benchparams.append( ("conv2d", conv2d_4_2_2) )
        benchparams.append( ("conv2d", conv2d_4_2_3) )

    benchparams.append( ("conv2d", conv2d_5_1_1) )
    benchparams.append( ("conv2d", conv2d_5_1_2) )
    benchparams.append( ("conv2d", conv2d_5_1_3) )
    benchparams.append( ("conv2d", conv2d_5_1_downsample) )
    for i in range(config[3]):
        benchparams.append( ("conv2d", conv2d_5_2_1) )
        benchparams.append( ("conv2d", conv2d_5_2_2) )
        benchparams.append( ("conv2d", conv2d_5_2_3) )

    benchparams.append( ("avgpooling", avgpooling_0) )
    denselayers = benchmarks["fulllayer"]
    benchparams.append( ("fulllayer", denselayers[0]) )

    return benchparams

def run_resnet50(benchmarks):
    return run_resnet([2,3,5,2],benchmarks)
def run_resnet101(benchmarks):
    return run_resnet([2,3,22,2],benchmarks)
def run_resnet152(benchmarks):
    return run_resnet([2,7,35,2],benchmarks)
