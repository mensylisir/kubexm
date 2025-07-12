package pipeline

// 删除BasePipeline抽象，每个具体pipeline直接实现Pipeline接口  
// 按照设计原则：pipeline层就是组装module为图，也别做过多抽象