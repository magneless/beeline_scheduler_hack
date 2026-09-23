//go:build ortools

package optimized

import cs "github.com/airspacetechnologies/or-tools/go/ortools/constraintsolver"

// Airspace's SWIG API accepts protobuf messages by value. Copying an unmarshaled
// message also copies protoimpl.MessageState's mutex. Construct fresh messages
// from their public fields instead. Nested messages stay behind pointers and
// remain private to this solve. Fields match the pinned d763a63d6ff2 binding.
func routingModelParameters(p *cs.RoutingModelParameters) cs.RoutingModelParameters {
	return cs.RoutingModelParameters{
		SolverParameters:       p.SolverParameters,
		ReduceVehicleCostModel: p.ReduceVehicleCostModel,
		MaxCallbackCacheSize:   p.MaxCallbackCacheSize,
	}
}
func routingSearchParameters(p *cs.RoutingSearchParameters) cs.RoutingSearchParameters {
	return cs.RoutingSearchParameters{
		FirstSolutionStrategy:                                  p.FirstSolutionStrategy,
		UseUnfilteredFirstSolutionStrategy:                     p.UseUnfilteredFirstSolutionStrategy,
		SavingsParameters:                                      p.SavingsParameters,
		GlobalCheapestInsertionFirstSolutionParameters:         p.GlobalCheapestInsertionFirstSolutionParameters,
		GlobalCheapestInsertionLsOperatorParameters:            p.GlobalCheapestInsertionLsOperatorParameters,
		LocalCheapestInsertionParameters:                       p.LocalCheapestInsertionParameters,
		LocalCheapestCostInsertionParameters:                   p.LocalCheapestCostInsertionParameters,
		ChristofidesUseMinimumMatching:                         p.ChristofidesUseMinimumMatching,
		FirstSolutionOptimizationPeriod:                        p.FirstSolutionOptimizationPeriod,
		LocalSearchOperators:                                   p.LocalSearchOperators,
		LsOperatorNeighborsRatio:                               p.LsOperatorNeighborsRatio,
		LsOperatorMinNeighbors:                                 p.LsOperatorMinNeighbors,
		UseMultiArmedBanditConcatenateOperators:                p.UseMultiArmedBanditConcatenateOperators,
		MultiArmedBanditCompoundOperatorMemoryCoefficient:      p.MultiArmedBanditCompoundOperatorMemoryCoefficient,
		MultiArmedBanditCompoundOperatorExplorationCoefficient: p.MultiArmedBanditCompoundOperatorExplorationCoefficient,
		MaxSwapActiveChainSize:                                 p.MaxSwapActiveChainSize,
		RelocateExpensiveChainNumArcsToConsider:                p.RelocateExpensiveChainNumArcsToConsider,
		HeuristicExpensiveChainLnsNumArcsToConsider:            p.HeuristicExpensiveChainLnsNumArcsToConsider,
		HeuristicCloseNodesLnsNumNodes:                         p.HeuristicCloseNodesLnsNumNodes,
		LocalSearchMetaheuristic:                               p.LocalSearchMetaheuristic,
		LocalSearchMetaheuristics:                              p.LocalSearchMetaheuristics,
		NumMaxLocalOptimaBeforeMetaheuristicSwitch:             p.NumMaxLocalOptimaBeforeMetaheuristicSwitch,
		GuidedLocalSearchLambdaCoefficient:                     p.GuidedLocalSearchLambdaCoefficient,
		GuidedLocalSearchResetPenaltiesOnNewBestSolution:       p.GuidedLocalSearchResetPenaltiesOnNewBestSolution,
		GuidedLocalSearchPenalizeWithVehicleClasses:            p.GuidedLocalSearchPenalizeWithVehicleClasses,
		UseGuidedLocalSearchPenaltiesInLocalSearchOperators:    p.UseGuidedLocalSearchPenaltiesInLocalSearchOperators,
		UseDepthFirstSearch:                                    p.UseDepthFirstSearch,
		UseCp:                                                  p.UseCp,
		UseCpSat:                                               p.UseCpSat,
		UseGeneralizedCpSat:                                    p.UseGeneralizedCpSat,
		SatParameters:                                          p.SatParameters,
		ReportIntermediateCpSatSolutions:                       p.ReportIntermediateCpSatSolutions,
		FallbackToCpSatSizeThreshold:                           p.FallbackToCpSatSizeThreshold,
		ContinuousSchedulingSolver:                             p.ContinuousSchedulingSolver,
		MixedIntegerSchedulingSolver:                           p.MixedIntegerSchedulingSolver,
		DisableSchedulingBewareThisMayDegradePerformance:       p.DisableSchedulingBewareThisMayDegradePerformance,
		OptimizationStep:                                       p.OptimizationStep,
		NumberOfSolutionsToCollect:                             p.NumberOfSolutionsToCollect,
		SolutionLimit:                                          p.SolutionLimit,
		TimeLimit:                                              p.TimeLimit,
		LnsTimeLimit:                                           p.LnsTimeLimit,
		SecondaryLsTimeLimitRatio:                              p.SecondaryLsTimeLimitRatio,
		ImprovementLimitParameters:                             p.ImprovementLimitParameters,
		UseFullPropagation:                                     p.UseFullPropagation,
		LogSearch:                                              p.LogSearch,
		LogCostScalingFactor:                                   p.LogCostScalingFactor,
		LogCostOffset:                                          p.LogCostOffset,
		LogTag:                                                 p.LogTag,
		UseIteratedLocalSearch:                                 p.UseIteratedLocalSearch,
		IteratedLocalSearchParameters:                          p.IteratedLocalSearchParameters,
	}
}
