def init_vgg16():
    benchmarks = dict()
    benchmarks["conv2d"] = []
    batch=1
    enable_backward=False
    convparams=[                                                     ##group inner
             [ batch,   3,224,224,64 ,3,3,1,1,1,1,enable_backward ],  ## 0,   0
             [ batch,  64,224,224,64 ,3,3,1,1,1,1,enable_backward ],  ## 0,   1
             [ batch,  64,112,112,128,3,3,1,1,1,1,enable_backward ],  ## 1,   0
             [ batch, 128,112,112,128,3,3,1,1,1,1,enable_backward ],  ## 1,   1,2
             [ batch, 128, 56, 56,256,3,3,1,1,1,1,enable_backward ],  ## 2,   0
             [ batch, 256, 56, 56,256,3,3,1,1,1,1,enable_backward ],  ## 2,   1,2
             [ batch, 256, 28, 28,512,3,3,1,1,1,1,enable_backward ],  ## 3,   0
             [ batch, 512, 28, 28,512,3,3,1,1,1,1,enable_backward ],  ## 3,   1,2
             [ batch, 512, 14, 14,512,3,3,1,1,1,1,enable_backward ],  ## 4,   0,1,2
            ]
    for param in convparams:
        cmd = "./conv2d -N %d -C %d -H %d -W %d -output-channel %d -kernel-height %d -kernel-width %d -pad-x %d -pad-y %d -stride-x %d -stride-y %d"%(param[0],param[1],param[2],param[3],param[4],param[5],param[6],param[7],param[8],param[9],param[10])
        if param[11]:
            cmd += " -enable-backward"
        benchmarks["conv2d"].append( cmd )
    benchmarks["maxpooling"] = []
    params=[                                                     ##group inner
             [ batch,  64,224,224, 64 ,2,2,0,0,2,2,enable_backward ],  ## 0,   3
             [ batch, 128,112,112,128 ,2,2,0,0,2,2,enable_backward ],  ## 1,   4
             [ batch, 256, 56, 56,256 ,2,2,0,0,2,2,enable_backward ],  ## 2,   4
             [ batch, 512, 28, 28,512 ,2,2,0,0,2,2,enable_backward ],  ## 3,   4
             [ batch, 512, 14, 14,512 ,2,2,0,0,2,2,enable_backward ],  ## 4,   4
            ]
    for param in params:
        cmd = "./maxpooling -N %d -C %d -H %d -W %d -output-channel %d -kernel-height %d -kernel-width %d -pad-x %d -pad-y %d -stride-x %d -stride-y %d"%(param[0],param[1],param[2],param[3],param[4],param[5],param[6],param[7],param[8],param[9],param[10])
        if param[11]:
            cmd += " -enable-backward"
        benchmarks["maxpooling"].append( cmd )
    
    benchmarks["fulllayer"] = []
    params=[                                                     ##group inner
             [ batch,  512*7*7, 512*2*2,enable_backward ],       ## 5,   0
             [ batch,  512*2*2,     200,enable_backward ],       ## 5,   1
            ]
    for param in params:
        cmd = "./fulllayer -N %d -input-dim %d -output-dim %d "%(param[0],param[1],param[2])
        if param[-1]:
            cmd += " -enable-backward"
        benchmarks["fulllayer"].append( cmd )
    return benchmarks
def run_vgg16(benchmarks):
    benchparams = [] 
    conv2ds = benchmarks["conv2d"]
    conv2d_0_0 = conv2ds[0]
    conv2d_0_1 = conv2ds[1]
    conv2d_1_0 = conv2ds[2]
    conv2d_1_1 = conv2ds[3]
    conv2d_1_2 = conv2ds[3]
    conv2d_2_0 = conv2ds[4]
    conv2d_2_1 = conv2ds[5]
    conv2d_2_2 = conv2ds[5]
    conv2d_3_0 = conv2ds[6]
    conv2d_3_1 = conv2ds[7]
    conv2d_3_2 = conv2ds[7]
    conv2d_4_0 = conv2ds[8]
    conv2d_4_1 = conv2ds[8]
    conv2d_4_2 = conv2ds[8]
    maxpoolings = benchmarks[ "maxpooling" ]
    maxpooling_0 = maxpoolings[0]
    maxpooling_1 = maxpoolings[1]
    maxpooling_2 = maxpoolings[2]
    maxpooling_3 = maxpoolings[3]
    maxpooling_4 = maxpoolings[4]
    benchparams.append( ("conv2d", conv2d_0_0) )
    benchparams.append( ("conv2d", conv2d_0_1) )
    benchparams.append( ("maxpooling", maxpooling_0) )
    benchparams.append( ("conv2d", conv2d_1_0) )
    benchparams.append( ("conv2d", conv2d_1_1) )
    benchparams.append( ("conv2d", conv2d_1_2) )
    benchparams.append( ("maxpooling", maxpooling_1) )
    benchparams.append( ("conv2d", conv2d_2_0) )
    benchparams.append( ("conv2d", conv2d_2_1) )
    benchparams.append( ("conv2d", conv2d_2_2) )
    benchparams.append( ("maxpooling", maxpooling_2) )
    benchparams.append( ("conv2d", conv2d_3_0) )
    benchparams.append( ("conv2d", conv2d_3_1) )
    benchparams.append( ("conv2d", conv2d_3_2) )
    benchparams.append( ("maxpooling", maxpooling_3) )
    benchparams.append( ("conv2d", conv2d_4_0) )
    benchparams.append( ("conv2d", conv2d_4_1) )
    benchparams.append( ("conv2d", conv2d_4_2) )
    benchparams.append( ("maxpooling", maxpooling_4) )
    denselayers = benchmarks["fulllayer"]
    benchparams.append( ("fulllayer", denselayers[0]) )
    benchparams.append( ("fulllayer", denselayers[1]) )
    return benchparams
