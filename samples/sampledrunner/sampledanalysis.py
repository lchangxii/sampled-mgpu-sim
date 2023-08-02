from sampledipc import processjson

import numpy as np

import math
def normalize(v):
    sumary = 0
    #print(v)
    for elem in v:
        sumary += elem * elem
    norm = math.sqrt(sumary)
    newv=[0]*len(v)
    for idx,elem in enumerate(v):
        newv[idx] = elem / norm
    return newv
threshold = 0.05
import numpy as np
class GPUBBV:
    def __init__(self,bbveachwarp):
        bbv2idx = dict()
        bbvvec = []
        self.bbvcount = []
        bbvdim = 0
        for bbv in bbveachwarp:
            bbv = bbv["Bbv"]
            value = hash(tuple(bbv))
            if value not in bbv2idx:
                bbv2idx[value] = len(bbv2idx)
                bbvvec.append(bbv)
                self.bbvcount.append(1)
                bbvdim += len(bbv)
            else:
                idx = bbv2idx[value]
                self.bbvcount[idx] += 1
        bbvtypenum = len(bbv2idx)
        bbvdatanum = len(bbveachwarp)
        bbvrate = [0] * bbvtypenum
        for idx in range(len(self.bbvcount)):
            bbvrate[idx] = 1.0 * self.bbvcount[idx] / bbvdatanum
        order = np.argsort( bbvrate)
        self.bbvrate = [0] * bbvtypenum
        i = 0
        self.bbvvec = []
        for idx in reversed(order):
            self.bbvrate[ i ] = bbvrate[idx]
            self.bbvvec.append(bbvvec[idx])
            i+=1
        ###generate gpu bbv
        self.gpubbv = []
        for idx,bbv in enumerate(self.bbvvec):
            bbvnorm = normalize(bbv)
            self.gpubbv += [ self.bbvrate[idx] * elem for elem in bbvnorm ]
    def __str__(self):
        return str(self.gpubbv)
    def __hash__(self):
        return hash(tuple(self.gpubbv))
    def dis(self,other):
        if len(self.gpubbv) < len( other.gpubbv):
            gpubbv1 = self.gpubbv + ( [0] * (len(other.gpubbv) - len(self.gpubbv)  ))
            gpubbv2 = other.gpubbv
        elif len(self.gpubbv) == len(other.gpubbv):
            gpubbv1 = self.gpubbv
            gpubbv2 = other.gpubbv
        else:
            gpubbv1 = self.gpubbv
            gpubbv2 = other.gpubbv + ( [0] * (len(self.gpubbv) - len(other.gpubbv)  ))
            
        dis = math.dist( gpubbv1 , gpubbv2)
        print(dis)
        if dis < threshold:
            return dis,True
        else:
            return dis,False

warpfullsize = 4 * 16 * 4 * 10 
class KernelInfo:
    def __init__( self,insnums,bbvs,warpnum,simtime=None ):
        self.gpubbv = GPUBBV(bbvs)
        self.insnums = sum(insnums)
        self.warpnum = warpnum
        if warpnum != len(insnums):
            self.insnums = self.insnums * 1.0 / len(insnums) * warpnum
        

        self.updatesim( simtime )
    def updatesim(self,simtime=None):
        self.simtime = simtime
        if simtime != None:
            self.cpi=self.simtime / self.insnums

    def predict(self,kernel_in_detailed):
        self.simtime = kernel_in_detailed.cpi * self.insnums
        return self.simtime
    def dis( self, kernel_record ):
        if self.warpnum < warpfullsize or kernel_record.warpnum < warpfullsize:
            if self.warpnum == kernel_record.warpnum:
                return self.gpubbv.dis( kernel_record.gpubbv )
            else:
                return 0,False
        else:
            return self.gpubbv.dis(kernel_record.gpubbv)
    def __hash__(self):
        return hash( tuple( [hash(self.gpubbv),hash(self.insnums)] ))

class ExecuteEngine:
    def __init__(self):
        self.kernel_infos = []
    def find_kernel(self, kernel ): 
        minimumdis = 1
        hitkernel = None
        for kernel_record in reversed(self.kernel_infos):
            if kernel_record.warpnum == kernel.warpnum:
                dis,hit = kernel.dis(kernel_record)
                if hit:
                    if dis < minimumdis:
                        hitkernel = kernel_record
                        minimumdis = dis
                    else:
                        pass
        if hitkernel == None:
            for kernel_record in reversed(self.kernel_infos):
                if kernel_record.warpnum != kernel.warpnum:
                    dis,hit = kernel.dis(kernel_record)
                    if hit:
                        if hitkernel != None:
                            rate1 = math.fabs(hitkernel.warpnum - kernel.warpnum)
                            rate2 = math.fabs(kernel_record.warpnum - kernel.warpnum)
                            if rate1 > rate2 :
                                hitkernel = kernel_record
                                minimumdis = dis
                        else:
                            hitkernel = kernel_record
#                        if dis < minimumdis:
 #                           hitkernel = kernel_record
  #                          minimumdis = dis
#                        else:
 #                           pass
        if hitkernel != None:
#            print(math.dist(kernel.gpubbv.gpubbv,hitkernel.gpubbv.gpubbv))
            #return kernel.predict(hitkernel),True
            return hitkernel,True
        else:
            return None,False

    def find_similar_kernel(self,kernel):
        hitkernel,suc = self.find_kernel(kernel)
        if suc:
            return kernel.predict(hitkernel),True
        else:
            return 0,False

    def update(self, kernel_record) :
        self.kernel_infos.append(kernel_record)

    def decompose_full_data( self, full_name ):
        data_full = processjson(full_name)
        print(full_name)
        simtimes = data_full["simtimes"]
        walltimes = data_full["walltimes"]
        return simtimes,walltimes
    def decompose_data( self, analysis_name ):
        analysis_data = processjson(analysis_name)
        bbvs_eachkernel = analysis_data[ "bbvs" ]
        insnumeachkernel = analysis_data["insnums"]
        warpseachkernel = analysis_data["wfnums"]
        analysistimes = analysis_data["analysistimes"]
        kernel_infos = []
        walltimes = []
        for idx, bbvkernel in enumerate(bbvs_eachkernel):
            insnumkernel = insnumeachkernel[idx]
            warpkernel = warpseachkernel[idx]
            kernel_info = KernelInfo( insnumkernel,bbvkernel,warpkernel )
            kernel_infos.append(kernel_info)
            walltimes.append( analysistimes[idx] )
        return kernel_infos,walltimes

import numpy as np
def bbvanalysis( fullname, filename ):
    data_full = processjson(fullname)
    data_analysis = processjson(filename)
    print(filename)
    bbveachkernel = data_analysis["bbvs"]
    insnumeachkernel = data_analysis["insnums"]
    simtimes = data_full["simtimes"]
    walltimes = data_full["walltimes"]
    ipcs = []
    features = []
    for idx in range(len( simtimes )) :
        simtime = simtimes[ idx ]
        insnum = sum(insnumeachkernel[idx])
        ipc = insnum / (simtime*1e9)
        ipcs.append(ipc)
        bbveachwarp = bbveachkernel[idx]
        #bbv_class = []
        global_bbvnum = dict()
        global_bbv = dict()
        allbbvsize = len(bbveachwarp)
        for bbv in bbveachwarp:
            bbv = bbv["Bbv"]
            value = hash(tuple(bbv))
            if value not in global_bbvnum:
                global_bbvnum[value] = 1
                global_bbv[value] = bbv
            else:
                global_bbvnum[value] = global_bbvnum[value] + 1
        globel_vector = []
        for elem,data in global_bbvnum.items():
            elem = global_bbv[elem]
            vec = normalize(elem)
            rate = data * 1.0 / allbbvsize
            for idx,data in enumerate(vec):
                vec[ idx ] *= rate
            globel_vector += vec
        globel_vector = normalize(globel_vector)
        #data = np.linalg.norm(globel_vector,0)
#        data = np.linalg.norm(globel_vector,2)
        #data = sum(globel_vector)
#        features.append(globel_vector)
        features.append(len(bbveachwarp))
    return ipcs,features
if __name__ == "__main__":
    another="/home/lcx/gpudata/bbv_feature___fulllayer__N_1__input_dim_2048__output_dim_200___magic_memory_copy__v1.json",
    bbvanalysis( "/home/lcx/gpudata/bbv_feature___fulllayer__N_1__input_dim_2048__output_dim_200___magic_memory_copy__only_analysis_v1.json","" )  

