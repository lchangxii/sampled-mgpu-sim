def init_resnet34():
    benchmarks = dict()
    benchmarks["conv2d"] = []
    batch=1
    enable_backward=False
                    ## in,  w,  h, co,kwh,pwh,stridewh
    convparams=[                                                     ##group       inner innerinner
             [ batch,    3,224,224,  64,7,7,3,3,2,2,enable_backward ],  ## 1
             [ batch,   64, 56, 56,  64,3,3,1,1,1,1,enable_backward ],  ## 2,        1/2,   1/2
    
             [ batch,   64, 56, 56, 128,3,3,1,1,2,2,enable_backward ],  ## 3,        1,     1
             [ batch,  128, 28, 28, 128,3,3,1,1,1,1,enable_backward ],  ## 3,        1,     2           2(1/2)
             [ batch,   64, 56, 56, 128,1,1,0,0,2,2,enable_backward ],  ## 3,        1,     downsample
    
             [ batch,  128, 28, 28, 256,3,3,1,1,2,2,enable_backward ],  ## 4,        1,     1
             [ batch,  256, 14, 14, 256,3,3,1,1,1,1,enable_backward ],  ## 4,        1,     2           2(1/2)
             [ batch,  128, 28, 28, 256,1,1,0,0,2,2,enable_backward ],  ## 4,        1,     downsample
    
             [ batch,  256, 14, 14, 512,3,3,1,1,2,2,enable_backward ],  ## 5,        1,     1
             [ batch,  512,  7,  7, 512,3,3,1,1,1,1,enable_backward ],  ## 5,        1,     2           2(1/2)
             [ batch,  256, 14, 14, 512,1,1,0,0,2,2,enable_backward ],  ## 5,        1,     downsample
    
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
             [ batch, 512,7,7, 512 ,7,7,0,0,1,1,enable_backward ],  ## 6
            ]
    for param in params:
        cmd = "./avgpooling -N %d -C %d -H %d -W %d -output-channel %d -kernel-height %d -kernel-width %d -pad-x %d -pad-y %d -stride-x %d -stride-y %d"%(param[0],param[1],param[2],param[3],param[4],param[5],param[6],param[7],param[8],param[9],param[10])
        if param[11]:
            cmd += " -enable-backward"
        benchmarks["avgpooling"].append( cmd )
    
    
    
    benchmarks["fulllayer"] = []
    params=[                                               ##group inner
             [ batch,  512, 1000,enable_backward ],       ## 7,   0
            ]
    for param in params:
        cmd = "./fulllayer -N %d -input-dim %d -output-dim %d "%(param[0],param[1],param[2])
        if param[-1]:
            cmd += " -enable-backward"
        benchmarks["fulllayer"].append( cmd )
    return benchmarks
def run_resnet34(benchmarks):
    benchparams = [] 
    conv2ds = benchmarks["conv2d"]
    conv2d_1_1_1 = conv2ds[0]
    
    conv2d_2_1_1 = conv2ds[1]
    conv2d_2_1_2 = conv2ds[1]
    conv2d_2_2_1 = conv2ds[1]
    conv2d_2_2_2 = conv2ds[1]
    
    conv2d_3_1_1 = conv2ds[2]
    conv2d_3_1_2 = conv2ds[3]
    conv2d_3_1_downsample = conv2ds[4]
    conv2d_3_2_1 = conv2ds[3]
    conv2d_3_2_2 = conv2ds[3]

    conv2d_4_1_1 = conv2ds[4]
    conv2d_4_1_2 = conv2ds[5]
    conv2d_4_1_downsample = conv2ds[6]
    conv2d_4_2_1 = conv2ds[5]
    conv2d_4_2_2 = conv2ds[5]

    conv2d_5_1_1 = conv2ds[7]
    conv2d_5_1_2 = conv2ds[8]
    conv2d_5_1_downsample = conv2ds[9]
    conv2d_5_2_1 = conv2ds[8]
    conv2d_5_2_2 = conv2ds[8]

    avgpoolings = benchmarks[ "avgpooling" ]
    avgpooling_0 = avgpoolings[0]
    maxpoolings = benchmarks[ "maxpooling" ]
    maxpooling_0 = maxpoolings[0]

    benchparams.append( ("conv2d", conv2d_1_1_1) )
    benchparams.append( ("maxpooling", maxpooling_0) )

    benchparams.append( ("conv2d", conv2d_2_1_1) )
    benchparams.append( ("conv2d", conv2d_2_1_2) )
    for i in range(2):
        benchparams.append( ("conv2d", conv2d_2_2_1) )
        benchparams.append( ("conv2d", conv2d_2_2_2) )

    benchparams.append( ("conv2d", conv2d_3_1_1) )
    benchparams.append( ("conv2d", conv2d_3_1_2) )
    benchparams.append( ("conv2d", conv2d_3_1_downsample) )
    for i in range(3):
        benchparams.append( ("conv2d", conv2d_3_2_1) )
        benchparams.append( ("conv2d", conv2d_3_2_2) )

    benchparams.append( ("conv2d", conv2d_4_1_1) )
    benchparams.append( ("conv2d", conv2d_4_1_2) )
    benchparams.append( ("conv2d", conv2d_4_1_downsample) )
    for i in range(5):
        benchparams.append( ("conv2d", conv2d_4_2_1) )
        benchparams.append( ("conv2d", conv2d_4_2_2) )

    benchparams.append( ("conv2d", conv2d_5_1_1) )
    benchparams.append( ("conv2d", conv2d_5_1_2) )
    benchparams.append( ("conv2d", conv2d_5_1_downsample) )
    for i in range(2):
        benchparams.append( ("conv2d", conv2d_5_2_1) )
        benchparams.append( ("conv2d", conv2d_5_2_2) )

    benchparams.append( ("avgpooling", avgpooling_0) )

    denselayers = benchmarks["fulllayer"]
    benchparams.append( ("fulllayer", denselayers[0]) )
    return benchparams
