package lpo

// psf: PreSolving Functions
//
// Auxiliary functions for LPO which reduce the original problem set by
// deleting rows and columns as per Andersen and Andersen paper (1993).
//
// The primary function is SolveProb, which in turn call other functions
// visible only within lpo to modify the data that was read and delete whatever
// rows and columns can be removed in order to simplify the problem. Components
// are added to this function as they are developed.

import (
	"encoding/xml"
	"fmt"
	"github.com/pkg/errors"
	"github.com/go-opt/gpx"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)


// PsCtrl specifies which components should be invoked, and how many iterations
// should be performed by the various components when reducing the size of the
// LP model. It is passed as an argument to SolveProb.
type PsCtrl struct {
	FileInMps         string  // MPS input file, or "" for none
	FileOutCplexSoln  string  // Cplex solution (xml) output file, or "" for none
	FileOutMpsRdcd    string  // Reduced MPS output file, or "" for none
	FileOutPsop       string  // Output file of pre-solve operations, or "" for none
	MaxIter           int     // Maximum iterations for lpo
	DelRowNonbinding  bool    // Controls if non-binding rows are removed
	DelRowSingleton   bool    // Controls if row singletons are removed
	DelColSingleton   bool    // Controls if column singletons are removed
	DelFixedVars      bool    // Controls if fixed variables are removed
	RunCplex          bool    // Controls if problem is to be solved by Cplex 		
}

// PsSoln returns the results from SolveProb to the caller. It contains the
// value of the objective function, the row and column maps of the LP, and the 
// numbers of rows, columns, and elements that were removed during presolve
// operations.
type PsSoln struct{
	ObjVal  float64       // Value of the objective function
	ConMap  PsResConMap   // Map of string to structs for constraints
	VarMap  PsResVarMap   // Map of string to structs for variables   
	RowsDel int           // Number of rows removed during presolve
	ColsDel int	          // Number of columns removed during presolve
	ElemDel int           // Number of elements removed during presolve	
}

// PsResConMap contains the map of constraints included in PsSoln that is
// returned to the caller. It contains values calculated by Cplex where available,
// or Status = NA and zero values if constaint was removed during presolve and not
// processed by Cplex.
type PsResConMap map[string] struct {
	Status      string    // Status from cplex, "NA" if not available
	Type        string    // Inequality type provided as model input
	Rhs         float64   // RHS provided as model input
	ScaleFactor float64   // Row scale factor used in original model
	Pi          float64   // Dual solution (Pi) from cplex, 0 if not available
	Slack       float64   // Slack from cplex, 0 if not available
	Dual        float64   // Dual from cplex, 0 if not available
}

// PsResVarMap contains the map of variables included in PsSoln that is returned to
// the caller. It contains values calculated by Cplex where available,
// or Status = NA and zero values if constaint was removed during presolve and not
// processed by Cplex.
type PsResVarMap map[string] struct {
	Status      string    // Status from cplex, "NA" if postsolved		
	Value       float64   // Variable value from cplex or postsolve calculation
	ScaleFactor float64   // Variable scale factor used in original model
	ReducedCost float64   // Reduced cost from cplex, 0 if postsolved
}

// psOp is used internally to record the presolve operation performed
// and the row and/or column that was removed during that operation.
type psOp struct {
	OpType  string  // Type of reduction operation performed 
	Col     psCol   // Column deleted from model by this operation (may be nil)
	Row     psRow   // Row deleted from model by this operation (may be nil)
}

// psRow is used internally in the list of presolve operations (psOp) to store
// data about the row that was removed.
type psRow struct {
	Name        string   // Row name
	Type        string   // Row type
	Rhs         float64  // Row RHS
	ScaleFactor float64  // Row scale factor
	Coef       []psCoef  // List of coefficients and variables for this row	
}

// psCoef is part of the psRow structure to store the list of variable names and
// their coefficients for a particular row (psRow) processed by presolve operations (psOp).
type psCoef struct {
	Name   string   // Variable name
	Value  float64  // Coefficient value
}

// psCol is used internally in the list of presolve operations (psOp) to store
// data about the column that was removed.
type psCol struct {
	Name        string   // Column name
	Type        string   // Column type
	BndLo       float64  // Lower bound
	BndUp       float64  // Upper bound
	ScaleFactor float64  // Column scale factor
}


// Constants used to determine which presolve operation was performed
const (
	psopFreeCol      = "FCS"   // Free and implied free column singleton
	psopFixedVar     = "FXV"   // Fixed variable
	psopRowSingltn   = "RSG"   // Row Singleton
	psopNbRow        = "NBR"   // Non-binding row
	psopEmptyRow     = "MTR"   // Empty row
)

// Delimiter for sections in PSOP file
const fileDelim = "#------------------------------------------------------------------------------\n"

const (
	psVarStatNA   = "NA"    // Var state provided by cplex not available
	psConStatNA   = "NA"    // Costr state provided by cplex not available
)

// Package global variables
var psOpList []psOp                     // Rows and cols deleted during presolve
var defaultCplexInput =  "cplexIn.txt"  // MPS file storing reduced matrix
var defaultCplexOutput = "cplexOut.txt" // File storing cplex solution


//==============================================================================
// GENERAL UTILITY AND POST-SOLVE FUNCTIONS
//==============================================================================

// getPstLhs returns the LHS of a constraint calculated from the constraint and 
// variables map passed to the function.
// In case of failure, it returns an error.
func getPstLhs(psRow psRow, psVarMap PsResVarMap, lhs *float64) error {

	*lhs = 0.0
		
	for i := 0; i < len(psRow.Coef); i++ {
		
		if psVar, ok := psVarMap[psRow.Coef[i].Name]; !ok {
			return errors.Errorf("Failed to find LHS for %s", psRow.Coef[i].Name) 
		} else {			
			*lhs += psVar.Value * psRow.Coef[i].Value		
		} 
	}

	// Adjust the calculated value by the scale factor.
	*lhs = *lhs * psRow.ScaleFactor
	
	return nil
}

//==============================================================================

// isMip checks if the problem is considered a MIP according to Cplex. It scans
// the columns, and if it detects any type other than "R" (which is translated
// to "continuous" for Cplex, it is considered a MIP, and the function returns
// "true". If it does not find one, it returns false and the problem can be solved
// as a pure linear problem (LP).
func isMip() bool {
	
	for i := 0; i < len(Cols); i++ {
		if Cols[i].Type != "R" {
			return true
		}
	}
	
	return false
}

//==============================================================================

// translateRow translates a single constraint (oldRow) to the psRow format 
// and returns it as newRow in the argument list.
// In case of failure, function returns an error.
func translateRow(oldRow InputRow, newRow *psRow) error {
	var iCurElem        int  // index of item in Elems list being processed
	var coef         psCoef  // coefficient item being processed
	var psCoefList []psCoef  // list of coefficients associated with this row

	newRow.Name = ""
	newRow.Type = ""
	newRow.Coef = nil
	newRow.Rhs = 0
	
	newRow.Name        = oldRow.Name
	newRow.Type        = oldRow.Type
	newRow.ScaleFactor = oldRow.ScaleFactor

	switch oldRow.Type {
	case "G", "E", "N":
		newRow.Rhs = oldRow.RHSlo

	case "L":
		newRow.Rhs = oldRow.RHSup

	} // End switch on row type
			

	for i := 0; i < len(oldRow.HasElems); i++ {

		iCurElem   = oldRow.HasElems[i]
		coef.Name  = Cols[Elems[iCurElem].InCol].Name
		coef.Value = Elems[iCurElem].Value
		psCoefList = append(psCoefList, coef)
	} // End for all coefficients in this row

	newRow.Coef = psCoefList

	return nil	
}

//==============================================================================

// translateAllRows returns the list of all rows (newRowList) in the psRow format 
// constructed from the original Rows, Cols, and Elems global variables. The new
// row format is needed to eliminate any cross references via indices to other
// lists, because the indices will change as rows and cols are deleted.
// In case of failure, function returns an error.
func translateAllRows(newRowList *[]psRow) error {
	var newRow psRow  // new row item being processed
	
	*newRowList = nil
	
	for i := 0; i < len(Rows); i++ {

		_      = translateRow(Rows[i], &newRow)
		*newRowList = append(*newRowList, newRow)	
	}
	
	return nil	
}

//==============================================================================

// updatePsList adds an item to the package global list of presolve operations
// (psOpList) which is constructed from the operation type (opType), as well as
// the row and/or column specified by rowIndex and colIndex, respectively,
// that are removed during the operation. A single row, a single column, or a row
// as well as a column may be removed.
// In case of failure, function returns an error.
func updatePsList(opType string, rowIndex int, colIndex int) error {
	
	var psItem psOp   // item associated with a post-solve operation list
	var err   error   // error returned by called functions
	
	// Transfer all variables "as is" from Cols data structures, if a column
	// was deleted.

	psItem.OpType          = opType	
	
	if colIndex >= 0 && colIndex < len(Cols) {
		psItem.Col.Name        = Cols[colIndex].Name
		psItem.Col.Type        = Cols[colIndex].Type
		psItem.Col.BndLo       = Cols[colIndex].BndLo
		psItem.Col.BndUp       = Cols[colIndex].BndUp
		psItem.Col.ScaleFactor = Cols[colIndex].ScaleFactor		
	} // End if a column was deleted	
	
	// If a row was deleted, translate it to the new format and add it to
	// the item that will be added to the list.

	if rowIndex >= 0 && rowIndex < len(Rows) {
		if err = translateRow(Rows[rowIndex], &psItem.Row); err != nil {
			return errors.Wrapf(err, "updatePsList failed with row %d", rowIndex)
		}
		
	} // End if row was deleted

	// Add the new item to the existing global list.	
	psOpList = append(psOpList, psItem)

	return nil
}

//==============================================================================

// buildVarMap returns the map of variables built from the cplex output. 
// In case of failure, it returns an error.
func buildVarMap(scaleMap map[string]float64, cpSoln []gpx.SolnCol, varbMap *PsResVarMap) error {

	newMap := make(PsResVarMap)	
		
	for i := 0; i < len(cpSoln); i++ {		

		scaleFactor, ok := scaleMap[cpSoln[i].Name];
		if !ok {
			return errors.Errorf("Missing scale factor for variable %s", cpSoln[i].Name)			
		}
		
		mapItem := newMap[cpSoln[i].Name]
		mapItem.Value       = cpSoln[i].Value
		mapItem.ScaleFactor = scaleFactor
		mapItem.ReducedCost = cpSoln[i].RedCost
		mapItem.Status      = psVarStatNA
					
		newMap[cpSoln[i].Name] = mapItem
	} // End for processing varbs list

	*varbMap = newMap
		
	return nil	
}

//==============================================================================

// addConMapItem updates the constraint map referenced by the psConMap argument
// by adding information from argument referenced by psRow.
// In case of failure, it returns an error.
func addConMapItem(psConMap PsResConMap, psRow psRow) error {

	if mapItem, ok := psConMap[psRow.Name]; !ok {
		// Item is new.
		// Initialize values which would be returned from cplex.
		mapItem.Status      = psConStatNA
		mapItem.Slack       = 0
		mapItem.Dual        = 0

		// Transfer values passed in
		mapItem.Rhs         = psRow.Rhs
		mapItem.Type        = psRow.Type
		mapItem.ScaleFactor = psRow.ScaleFactor
		
		psConMap[psRow.Name] = mapItem		
	} else {
		// Item exists, but we need to restore original Rhs and type which were
		// modified as a result of presolve.
		// Initialize values which would be returned from cplex.	
		mapItem.Status = psConMap[psRow.Name].Status
		mapItem.Slack  = psConMap[psRow.Name].Slack
		mapItem.Dual   = psConMap[psRow.Name].Dual

		// Transfer values passed in
		mapItem.Rhs         = psRow.Rhs
		mapItem.Type        = psRow.Type
		mapItem.ScaleFactor = psRow.ScaleFactor
			
		psConMap[psRow.Name] = mapItem			
	}

	return nil
		
}

//==============================================================================

// buildConMap returns the map of constraints built from the cplex output.
// In case of failure, function returns an error.
func buildConMap(cpSoln []gpx.SolnRow, constrMap *PsResConMap) error {
	
	newMap := make(PsResConMap)	
	
	for i := 0; i < len(cpSoln); i++ {
		
		mapItem := newMap[cpSoln[i].Name]

		// Transfer values received from cplex		
		mapItem.Status = psVarStatNA
		mapItem.Slack  = cpSoln[i].Slack
		mapItem.Pi     = cpSoln[i].Pi
		// TODO: The dual value is not automatically provided by CPX functions
		// as it is in the xml file, and would need to be obtained separately.
		mapItem.Dual   = 0

		// Initialize all other values that we may fill in later
		mapItem.Rhs    = 0
		mapItem.Type   = "X"
		
		newMap[cpSoln[i].Name] = mapItem
	}

	*constrMap = newMap	

	return nil
}

//==============================================================================

// postSolve updates the constraint map (pscMap) and variable map (solvedVarMap)
// referenced in the argument list with information stored in the package global
// list of presolve operations (psOpList). Effectively, this function merges the
// results calculated by Cplex with results obtained by "reversing" the presolve
// operations. In case of failure, function returns an error.
func postSolve(pscMap PsResConMap, solvedVarMap PsResVarMap) error {
	var rhs   float64  // RHS of row being processed
	var lhs   float64  // LHS of row being processed
	var coef  float64  // holder for value of coefficient being processed
	var curVar psCoef  // holder for variable structure being processed
		
	for i := len(psOpList) - 1; i >= 0; i-- {
	
		switch psOpList[i].OpType {

		// Operations recorded so they could be printed, but which don't need
		// any post-solve steps and can be ignored
		case psopEmptyRow, psopNbRow:
			continue
						
		// Fixed Variable ------------------------------------------------------	
		case psopFixedVar:

			// Calculate variable value and add it to solved variables map.
			// There is no row information to transfer for this item.
			varbMap := make(PsResVarMap)
			vMapItem := varbMap[psOpList[i].Col.Name]
			vMapItem.Value       = psOpList[i].Col.BndLo
			vMapItem.ReducedCost = 0
			vMapItem.Status      = psVarStatNA
			vMapItem.ScaleFactor = psOpList[i].Col.ScaleFactor
			solvedVarMap[psOpList[i].Col.Name] = vMapItem
				
		// Free Column Singleton -----------------------------------------------	
		case psopFreeCol:	

			// First get the RHS and coefficient value
			rhs = psOpList[i].Row.Rhs
			lhs = 0
			coef = 0
			for j := 0; j < len(psOpList[i].Row.Coef); j++ {
				curVar.Name = psOpList[i].Row.Coef[j].Name
				if curVar.Name == psOpList[i].Col.Name {
					// If this is the varb we are looking for, get coef. for this row
					coef = psOpList[i].Row.Coef[j].Value					
				} else {
					// If different varb, sum up its value times coef (lhs) for this row
					if psVar, ok := solvedVarMap[curVar.Name]; !ok {
						return errors.Errorf("postSolve unable to find value for %s", curVar.Name)
					} else {
						lhs += psVar.Value * psOpList[i].Row.Coef[j].Value
					} // End else increment lhs value
				} // End else not variable we need to solve
			} // End for all variables in row
			
			// Calculate variable value and add it to solved variables map
			varbMap             := make(PsResVarMap)
			vMapItem            := varbMap[psOpList[i].Col.Name]
			vMapItem.Value       = (rhs - lhs) / coef
			vMapItem.ReducedCost = 0
			vMapItem.Status      = psVarStatNA
			vMapItem.ScaleFactor = psOpList[i].Col.ScaleFactor
			solvedVarMap[psOpList[i].Col.Name] = vMapItem

			// Get deleted row details and add it to solved constraints map
			constrMap      := make(PsResConMap)
			cMapItem       := constrMap[psOpList[i].Row.Name]
			cMapItem.Type        = psOpList[i].Row.Type
			cMapItem.Rhs         = psOpList[i].Row.Rhs
			cMapItem.Dual        = 0
			cMapItem.Slack       = 0
			cMapItem.Status      = psConStatNA
			cMapItem.ScaleFactor = psOpList[i].Row.ScaleFactor
			pscMap[psOpList[i].Row.Name] = cMapItem
			

		// Row Singleton -------------------------------------------------------	
		case psopRowSingltn:

			rhs = psOpList[i].Row.Rhs
			coef = 0
			for j := 0; j < len(psOpList[i].Row.Coef); j++ {
				if psOpList[i].Row.Coef[j].Name == psOpList[i].Col.Name {
					coef = psOpList[i].Row.Coef[j].Value
					break
				} 
			}
			
			if coef == 0 {
				return errors.New("postSolve unable to find coefficient")
			}
			
			// Calculate variable value and add it to solved variables map
			varbMap             := make(PsResVarMap)
			vMapItem            := varbMap[psOpList[i].Col.Name]
			vMapItem.Value       = rhs / coef
			vMapItem.ReducedCost = 0
			vMapItem.Status      = psVarStatNA
			vMapItem.ScaleFactor = psOpList[i].Col.ScaleFactor
			solvedVarMap[psOpList[i].Col.Name] = vMapItem

			// Get deleted row details and add it to solved constraints map
			constrMap      := make(PsResConMap)
			cMapItem       := constrMap[psOpList[i].Row.Name]
			cMapItem.Type        = psOpList[i].Row.Type
			cMapItem.Rhs         = psOpList[i].Row.Rhs
			cMapItem.Dual        = 0
			cMapItem.Slack       = 0
			cMapItem.Status      = psConStatNA
			cMapItem.ScaleFactor = psOpList[i].Row.ScaleFactor
			pscMap[psOpList[i].Row.Name] = cMapItem
					

		// Something unknown ---------------------------------------------------	
		default:
			return errors.Errorf("Unexpected operation %s in postSolve", psOpList[i].OpType)
		} // End switch on operation type
		
	} // End for processing psOpList
	
	return nil
}

//==============================================================================
// ROW REDUCTION OPERATIONS
//==============================================================================

// swapRows switches the rows specified by source and destination indices 
// (srcIndex, destIndex) in Rows list and updates all cross-references.
// In case of failure, it returns an error.
func swapRows(srcIndex int, destIndex int) error {

	var tempRow InputRow  // holder for row being moved
	var index        int  // holder for index of item being processed

	// Return error if indices are out of range

	if srcIndex < 0 || srcIndex >= len(Rows) {
		return errors.Errorf("Source index %d out of range in swapRows", srcIndex)
	}

	if destIndex < 0 || destIndex >= len(Rows) {
		return errors.Errorf("Dest. index %d out of range in swapRows", destIndex)
	}

	// Swap the references to the two rows in the elements lists.

	for i := 0; i < len(Rows[srcIndex].HasElems); i++ {
		index = Rows[srcIndex].HasElems[i]
		Elems[index].InRow = destIndex
	}

	for i := 0; i < len(Rows[destIndex].HasElems); i++ {
		index = Rows[destIndex].HasElems[i]
		Elems[index].InRow = srcIndex
	}

	// Swap the two rows.

	tempRow         = Rows[destIndex]
	Rows[destIndex] = Rows[srcIndex]
	Rows[srcIndex]  = tempRow

	return nil
}


//==============================================================================

// DelRow deletes the row specified by index srcRow, and updates all cross references.
// If srcRow is not already the last row in the list, the row is swapped with the
// last row and is deleted from the end of the list.
// In case of failure, it returns an error.
func DelRow(srcRow int) error {

	var iCurElem       int   // index of current element
	var iLastElem      int   // index of last element in list
	var lastRow        int   // index of last row in list
	var index          int   // holder for index being processed
	var elemList     []int   // list of element associated with item
	var newElemList  []int   // new element list created after items deleted
	var tempElem InputElem   // temporary holder for element
	var err          error   // error received from called functions

	lastRow = len(Rows) - 1
	
	// Check that index of row to be deleted is valid.
	if srcRow < 0 || srcRow > lastRow {
		return errors.Errorf("Row index %d out of range", srcRow)
	}

	// If row to be deleted is not the last row, swap rows to put it at end of list.
	if srcRow != lastRow {
		if err = swapRows(srcRow, lastRow); err != nil {
			return errors.Wrap(err, "Row swap failed")
		}			
	}

	// Step through the list of elements in the row to be deleted and migrate
	// them to the end of the elements list by swapping with those that remain.
	// Use temporary elemList to keep track of all elements that need to be
	// processed because the HasElems list associated with the current row may be changing.
	
	iLastElem = len(Elems) - 1
	elemList  = Rows[lastRow].HasElems
	
	for i := 0; i < len(elemList); i++ {

		iCurElem = elemList[i]

		// Remove element to be deleted from the column where it occurs.
		index = Elems[iCurElem].InCol
		newElemList = nil
		for j := 0; j < len(Cols[index].HasElems); j++ {
			if Cols[index].HasElems[j] != iCurElem {
				newElemList = append(newElemList, Cols[index].HasElems[j])
			}
		}
		Cols[index].HasElems = newElemList
		
		// Find	the row location of the former last element and update reference.
		index = Elems[iLastElem].InRow
		for j := 0; j < len(Rows[index].HasElems); j++ {
			if Rows[index].HasElems[j] == iLastElem {
				Rows[index].HasElems[j] = iCurElem
				break
			}
		}

		// Find the column location of the former last element and update reference.
		index = Elems[iLastElem].InCol
		for j := 0; j < len(Cols[index].HasElems); j++ {
			if Cols[index].HasElems[j] == iLastElem {
				Cols[index].HasElems[j] = iCurElem
				break
			}
		}
		
		// Swap the elements and update index of next available slot.
		tempElem         = Elems[iLastElem]
		Elems[iLastElem] = Elems[iCurElem]
		Elems[iCurElem]  = tempElem
		iLastElem--	
		
	} // End for all elements of row being deleted.

	// Reslice the rows and elements lists.
	Elems = append(Elems[:iLastElem + 1])
	Rows  = append(Rows[:len(Rows) - 1])
	
	return nil
}

//==============================================================================

// DelCol deletes the column specified by srcCol and updates all cross references.
// If the column to be deleted is not already the last column in the list, it is
// swapped with the last column and is deleted from the end of the list.
// In case of failure, it returns an error.
func DelCol(srcCol int) error {
	var err          error  // error string returned from other functions
	var lastCol        int  // index of last column in the global list
	var iCurElem       int  // index of current element being processed
	var iLastElem      int  // index of last element in global list
	var index          int  // general variable for storing indices as needed
	var elemList     []int  // list of elements being processed
	var newElemList  []int  // new list excluding elements that were deleted
	var tempElem InputElem  // placeholder for swapping items in element list


	lastCol = len(Cols) - 1

	// Exit if index we received is out of range.
	if srcCol < 0 || srcCol > lastCol {
		return errors.Errorf("Column index %d out of range", srcCol)
	}

	// If column to be deleted is not already the last one, swap with last column.
	if srcCol != lastCol {
		if err = swapCols(srcCol, lastCol); err != nil {
			return errors.Wrap(err, "Column swap failed")			
		}	
	}
	
	// Step through the list of elements in the column to be deleted and migrate
	// them to the end of the elements list by swapping with those that remain.
	// Use temporary elemList to keep track of all elements that need to be
	// processed because the Elem list associated with the current column may be changing.
	
	iLastElem = len(Elems) - 1
	elemList  = Cols[lastCol].HasElems

	for i := 0; i < len(elemList); i++ {
	
		iCurElem = elemList[i]
				
		// Remove element to be deleted from row where it occurs.
		index       = Elems[iCurElem].InRow
		newElemList = nil
		for j := 0; j < len(Rows[index].HasElems); j++ {
			if Rows[index].HasElems[j] != iCurElem {
				newElemList = append(newElemList, Rows[index].HasElems[j])
			}	
		}
		Rows[index].HasElems = newElemList

		// Find	the row location of the former last element and update reference.
		index = Elems[iLastElem].InRow
		for j := 0; j < len(Rows[index].HasElems); j++ {
			if Rows[index].HasElems[j] == iLastElem {
				Rows[index].HasElems[j] = iCurElem
				break
			}
		}

		// Find the column location of the former last element and update reference.
		index = Elems[iLastElem].InCol
		for j := 0; j < len(Cols[index].HasElems); j++ {
			if Cols[index].HasElems[j] == iLastElem {
				Cols[index].HasElems[j] = iCurElem
				break
			}
		}
		
		// Swap the elements and update index of next available slot.
		tempElem          = Elems[iLastElem]
		Elems[iLastElem] = Elems[iCurElem]
		Elems[iCurElem]  = tempElem

		iLastElem--				
	}

	// Reslice the rows and elements lists.
	Elems = append(Elems[:iLastElem + 1])
	Cols  = append(Cols[:len(Cols) - 1])
	
	return nil
}

//==============================================================================

// delTaggedRows finds rows tagged for deletion, moves them to end of list by 
// swapping with still-active rows, deletes all tagged rows, and updates 
// cross-references. Function passes back the number of rows deleted in the numDltd
// variable.
// In case of failure, function returns an error.
func delTaggedRows(numDltd *int) error {
	var err error    // error received from called functions

	// Delete any rows tagged for deletion by looking from the end of the list.
	// Swapping of rows to put them at the end is done by the deletion function.
	for i := len(Rows) - 1; i >= 0; i-- {

		if Rows[i].State == stateDelete {

			if err = DelRow(i); err != nil {
				return errors.Wrapf(err, "Failed to delete row %d", i)
			} // End if row deletion failed

			*numDltd = *numDltd + 1

		} // End if found row to delete
	} // End for all rows in list

	return nil
}

//==============================================================================

// delNbRows searches the Rows list for any non-binding rows that are still
// in the active state and deletes them. It passes back the number of rows deleted
// in the numDltd variable.
// In case of failure, function returns an error.
func delNbRows(numDltd *int) error {
	var err error  // error received from called functions
		
	log(pINFO, "Looking for non-binding rows...\n")
	
	*numDltd = 0

	// Skip over rows if they are not active.	
	for i := 0; i < len(Rows); i++ {

		if Rows[i].State != stateActive {
			continue
		}
		
		if Rows[i].Type == "N" {
			Rows[i].State = stateDelete	
			_ = updatePsList(psopNbRow, i, -1)
		}	
	} // End for looping over all rows	

	if err = delTaggedRows(numDltd); err != nil {
		return errors.Wrap(err, "delNbRows failed")
	}
	
	if *numDltd != 0 {
		log(pINFO, "Deleted %d non-binding rows.\n", *numDltd)
	}
	
	return nil
}

//==============================================================================

// delEmptyRows searches the Rows list for any empty rows that are still
// in the active state and deletes them. It passes back the number of rows deleted
// in the numDltd variable.
// In case of failure, function returns an error.
func delEmptyRows(numDltd *int) error {
	var err error  // error received from called functions
		
	log(pINFO, "Looking for empty rows...\n")

	*numDltd = 0
	
	for i := 0; i < len(Rows); i++ {

		// Skip over any rows that are not still active or that are not empty	
		if  len(Rows[i].HasElems) > 0 || Rows[i].State != stateActive {
			continue
		}

		// Lower bounds may not be correct.	
		if Rows[i].RHSlo == -Plinfy && Rows[i].RHSup != 0 {
			log(pWARN, "WARNING: Empty row %s has bounds %f to %f.\n",
				Rows[i].Name, Rows[i].RHSlo, Rows[i].RHSup)
		}	

		// Upper bounds may not be correct.	
		if Rows[i].RHSlo != 0 && Rows[i].RHSup == Plinfy {
			log(pWARN, "WARNING: Empty row %s has bounds %f to %f.\n",
				Rows[i].Name, Rows[i].RHSlo, Rows[i].RHSup)
		}	

		Rows[i].State = stateDelete
		_ = updatePsList(psopEmptyRow, i, -1)
		log(pDEB, "  Row %s removed.\n", Rows[i].Name)
	
	} // End for all rows

	if err = delTaggedRows(numDltd); err != nil {
		return errors.Wrap(err, "delEmptyRows failed")
	}
	
	if *numDltd != 0 {
		log(pINFO, "Deleted %d empty rows.\n", *numDltd)		
	}

	return nil
}

//==============================================================================
// COLUMN REDUCTION OPERATIONS
//==============================================================================

// swapCols switches columns specified by source and destination indices 
// (srcIndex, destIndex) in Cols list and updates all cross-references.
// In case of failure, it returns an error.
func swapCols(srcIndex int, destIndex int) error {
	var tempCol InputCol  // temporary holder for column as we swap them
	var index        int  // temporary holder for index needed during processing

	// Return error if indices are out of range.

	if srcIndex < 0 || srcIndex >= len(Cols) {
		return errors.Errorf("Source index %d out of range in swapCols", srcIndex)
	}

	if destIndex < 0 || destIndex >= len(Cols) {
		return errors.Errorf("Destination index %d out of range in swapCols", destIndex)
	}

	// Swap references to the two columns in the elements lists.

	for i := 0; i < len(Cols[srcIndex].HasElems); i++ {
		index = Cols[srcIndex].HasElems[i]
		Elems[index].InCol = destIndex
	}

	for i := 0; i < len(Cols[destIndex].HasElems); i++ {
		index = Cols[destIndex].HasElems[i]
		Elems[index].InCol = srcIndex
	}

	// Swap the columns.

	tempCol         = Cols[destIndex]
	Cols[destIndex] = Cols[srcIndex]
	Cols[srcIndex]  = tempCol

	return nil
}

//==============================================================================

// delFixedVars deletes all fixed variables and passes back the number of columns 
// deleted in the numDltd variable.
// In case of failure, function returns an error.
func delFixedVars(numDltd *int) error {
	var index    int  // holder for index of list currently being processed
	var coef float64  // coefficient value
	var err    error  // error returned by secondary functions

	// Initialize variables
	*numDltd = 0

	log(pINFO, "Looking for fixed variables ...\n")
	
	// Look through the columns list for items that have same upper & lower bound
	for i := 0; i < len(Cols); i++ {

		// Only process active rows, and skip over locked or deleted ones.
		if Cols[i].State != stateActive {
			continue
		}
		
		if Cols[i].BndLo != Cols[i].BndUp {
			// Not a fixed variable, move to the next one.
			continue
		}

		// Tag the column for deletion and add it to the list of cols deleted.
		log(pDEB, "  Col %s removed.\n", Cols[i].Name)
		Cols[i].State = stateDelete				
		_ = updatePsList(psopFixedVar, -1, i)

		// Update RHS for each row where this column appears.
		for j := 0; j < len(Cols[i].HasElems); j++ {

			index = Elems[Cols[i].HasElems[j]].InRow
			coef  = Elems[Cols[i].HasElems[j]].Value

			if Rows[index].RHSlo != -Plinfy {
				Rows[index].RHSlo -= Cols[i].BndLo * coef
			}

			if Rows[index].RHSup != Plinfy {
				Rows[index].RHSup -= Cols[i].BndUp * coef
			}
			
		} // End for all rows associated with fixed variable
				
	} // End for all columns


	if err = delTaggedCols(numDltd); err != nil {
		return errors.Wrap(err, "delFixedVars failed")
	}
	
	if *numDltd != 0 {
		log(pINFO, "Deleted %d fixed variables.\n", *numDltd)		
	}
	
	return nil
}

//==============================================================================

// delTaggedCols finds columns tagged for deletion, moves them to end of list by 
// swapping with still-active columns, deletes all tagged columns, and updates 
// cross-references. Function passes back the number of columns deleted in the
// numDltd variable.
// In case of failure, function returns an error.
func delTaggedCols(numDltd *int) error {
	var err error  // error value received from called functions

	// Initialize variables
	*numDltd = 0

	// Delete all tagged columns, working from the end of the list. Swapping of
	// columns to put them at the end is done by the deletion function.

	for i := len(Cols) - 1; i >= 0; i-- {

		if Cols[i].State == stateDelete {
			if err = DelCol(i); err != nil {
				return errors.Wrapf(err, "Failed to delete column %d", i)				
			}

		*numDltd = *numDltd + 1
		
		} // End if column tagged for deletion
	} // End for deleting all tagged columns

	return nil
}

//==============================================================================

// delFreeColSingls deletes free column singletons, defined as a variable 
// which appears only in the objective function and has bounds from negative
// infinity to positive infinity. It passes the number of items (rows and cols) 
// deleted back in the numDltd variable.
// In case of failure, function returns an error.
func delFreeColSingls(numDltd *int) error {
	var rowIndex  int  // holder for index of list item currently being processed
	var rowsFound int  // number of rows found and deleted
	var colsFound int  // number of columns found and deleted
	var err     error  // error received from called functions

	*numDltd = 0
	rowIndex = -1
	
	for i := 0; i < len(Cols); i++ {

		if len(Cols[i].HasElems) != 1 {
			// Variable occurs in more than one place, can't be removed.
			continue
		}
		
		if Cols[i].BndLo != -Plinfy || Cols[i].BndUp != Plinfy {
			// Not a free variable, can't be removed.
			continue
		} 
		
		rowIndex =  Elems[Cols[i].HasElems[0]].InRow
		if rowIndex == ObjRow {
			// Variable occurs only in objective function, can't be removed.
			log(pDEB, "Variable %s in objective %s, not in any constraint.\n",
				Cols[i].Name, Rows[ObjRow].Name)
			continue
		}
		
		// Tag the column and row for deletion, and add them to postsolve list.
		
		log(pINFO, "  Row %s and col %s removed.\n", Rows[rowIndex].Name, Cols[i].Name)
		
		Cols[i].State = stateDelete
		Rows[rowIndex].State = stateDelete
		_ = updatePsList(psopFreeCol, rowIndex, i)
						
	} // End for all columns	

	// Delete the rows and columns, if row deletion fails return at that point,
	// otherwise delete columns and return with the appropriate return code and
	// number of items deleted.
	
	if err = delTaggedRows(&rowsFound); err != nil {
		*numDltd = rowsFound
		return errors.Wrap(err, "delFreeColSingls row deletion failed")	
	}
	
	err = delTaggedCols(&colsFound)
	*numDltd = rowsFound + colsFound
	if err != nil {
		return errors.Wrap(err, "delFreeColSingls col deletion failed")
	}
	
	if rowsFound != 0 || colsFound != 0 {
		log(pINFO, "Deleted %d rows and %d cols.\n", rowsFound, colsFound)
	}

	return nil	
}

//==============================================================================

// delRowSingletons searches the Rows list for any singleton rows that are still
// in the active state and deletes them. It passes the number of rows deleted back
// in the numDltd variable.
// In case of failure, function an error.
func delRowSingletons(numDltd *int) error {
	var colIndex     int  // column index of item being processed
	var rowIndex     int  // row index of item being processed
	var colsFound    int  // number of columns found and deleted
	var rowsFound    int  // number of rows found and deleted
	var coef     float64  // coefficient value
	var newBound float64  // updated RHS value for row being processed
	var err        error  // error received from secondary functions

	// Initialize variables
	*numDltd  = 0
	rowsFound = 0
	colsFound = 0

	log(pINFO, "Looking for row singletons...\n")
	
	// Find all rows that contain a single variable.

	for i := 0; i < len(Rows); i++ {

		// Only process active rows, skip over locked and deleted ones.		
		if Rows[i].State != stateActive {
			continue
		}
		
		if len(Rows[i].HasElems) == 1 {


			if Rows[i].Type != "E" {
				// Skip any inequalities.
				continue
			}
			
			colIndex = Elems[Rows[i].HasElems[0]].InCol
			coef     = Elems[Rows[i].HasElems[0]].Value
			//log(pTRC, "Found singleton row [%d-%s], col [%d-%s]\n",
			//	i, Rows[i].Name, colIndex, Cols[colIndex].Name)

			// Don't want any divisions by zero, so check just in case.
			if coef == 0 {
				log(pERR, "Error: Unexpected zero coef for Row %s, Col %s.\n",
					Rows[i].Name, Cols[colIndex].Name)
				return nil
			}

			// Set the variable bounds depending on the row type.
			switch Rows[i].Type {

			// TODO: Andersen algorithm may only work for equality constraints, and
			// the other cases can be removed. For now, leave them in until confirmed.
			case "G":
				newBound = Rows[i].RHSlo / coef
				Cols[colIndex].BndLo = newBound

			case "L":
				newBound = Rows[i].RHSup / coef
				Cols[colIndex].BndUp = newBound

			case "E", "N":
				newBound = Rows[i].RHSlo / coef
				Cols[colIndex].BndLo = newBound
				Cols[colIndex].BndUp = newBound

			} // End switch on row type

			
			// Adjust the RHS of each constraint where this variable occurs
			
			for j := 0; j < len(Cols[colIndex].HasElems); j++ {
				rowIndex =  Elems[Cols[colIndex].HasElems[j]].InRow
				coef     =  Elems[Cols[colIndex].HasElems[j]].Value
				//log(pTRC, "  j = %d, row %s\n", j, Rows[rowIndex].Name)
				
				if i == rowIndex {
					//log(pDEB, "Skipping target row [%d] %s\n", i, Rows[i].Name)
					continue
				}
				
				if Rows[rowIndex].RHSlo != -Plinfy {
					Rows[rowIndex].RHSlo -= newBound * coef
				}
				
				if Rows[rowIndex].RHSup != Plinfy {
					Rows[rowIndex].RHSup -= newBound	* coef
				}				
			} // End for all rows where this variable occurs
			

			// Delete row and column and update list and counters

			Rows[i].State        = stateDelete
			Cols[colIndex].State = stateDelete
			_ = updatePsList(psopRowSingltn, i, colIndex)
			log(pINFO, "  Row %s and col %s removed.\n", Rows[i].Name, Cols[colIndex].Name)
			//log(pTRC, "    Row lo %f up %f, var lo %f up %f\n", Rows[i].RHSlo, Rows[i].RHSup,
			//	Cols[colIndex].BndLo, Cols[colIndex].BndUp)
									
		} // End if we found singleton row
	} // End for all rows in the list

	// Delete the rows and columns, if row deletion fails return at that point,
	// otherwise delete columns and return with the appropriate return code and
	// number of items deleted.
	
	if err = delTaggedRows(&rowsFound); err != nil {
		*numDltd = rowsFound
		return errors.Wrap(err, "delRowSingeltons failed")	
	}

	err = delTaggedCols(&colsFound)
	*numDltd = rowsFound + colsFound
	if err != nil {
		return errors.Wrap(err, "delRowSingletons failed")
	}
	
	if rowsFound != 0 || colsFound != 0 {
		log(pINFO, "Deleted %d rows and %d cols.\n", rowsFound, colsFound)
	}

	return nil
}

//==============================================================================
// EXPORTED FUNCTIONS
//==============================================================================

// ReduceMatrix iteratively performs the reduction operations specified in the psControl 
// structure to remove rows and columns from the model until no more reductions
// occur, or until the maximum number of iterations is reached. The function also 
// performs some additional reductions (e.g. removal of empty rows) which are not configurable.
//
// In case of failure, the function returns an error.
//
//	The fields of the psControl structure have the following meaning for this function:
//	   MaxIter           int    - maximum iterations for reduction loop
//	   DelRowNonbinding  bool   - if true, remove non-binding rows
//	   DelRowSingleton   bool   - if true, remove row singletons
//	   DelColSingleton   bool   - if true, remove column singletons
//	   DelFixedVars      bool   - if true, remove fixed variables
//	   RunCplex          bool   - ignored by this function 		
//	   FileInMps         string - ignored by this function
//	   FileOutCplexSoln  string - ignored by this function
//	   FileOutMpsRdcd    string - ignored by this function
//	   FileOutPsop       string - ignored by this function
func ReduceMatrix(psControl PsCtrl) error {
	var itemsFound  int  // number of items deleted by a specific operation
	var itemsInPass int  // number of changes made in current iteration
	var numChanges  int  // number of changes made in all iterations
	var totalIter   int  // number of iterations performed by TightenBounds
	var err       error  // error returned by secondary functions called

	numChanges = 0
	
	for i := 1; i <= psControl.MaxIter; i++ {

		// Iterate over row and column reductions until no more changes in the
		// number of elements are observed. The NumElements global counter is 
		// updated whenever rows or columns are deleted.
		
		itemsInPass = 0
		
		log(pINFO, "\nIteration %d: %d rows, %d cols, %d elements.\n", i,
			len(Rows), len(Cols), len(Elems))

		if psControl.DelRowNonbinding {

			if err = TightenBounds(psControl.MaxIter, &totalIter); err != nil {
				return errors.Wrap(err, "TightenBounds failed")		
			}
			
			if err = delNbRows(&itemsFound); err != nil {
				numChanges += itemsFound
				return errors.Wrap(err, "ReduceMatrix failed")				
			}
					
			itemsInPass += itemsFound
		} // End if non-binding row


		if psControl.DelFixedVars || psControl.DelRowNonbinding {
			// This component must be executed if non-binding rows were removed.
			if err = delFixedVars(&itemsFound); err != nil {
				numChanges += itemsFound
				return errors.Wrap(err, "ReduceMatrix failed")
			}

			itemsInPass += itemsFound
		} // End if fixed variable


		if psControl.DelRowSingleton {
			if err = delRowSingletons(&itemsFound); err != nil {
				numChanges += itemsFound
				return errors.Wrap(err, "ReduceMatrix failed")							
			}
			
			itemsInPass += itemsFound			
		} // End if row singleton	

						
		if psControl.DelColSingleton {
			if err = delFreeColSingls(&itemsFound); err != nil {
				numChanges += itemsFound
				return errors.Wrap(err, "ReduceMatrix failed")								
			}
			
			itemsInPass += itemsFound						
		} // End if column singleton

		// Empty rows are deleted automatically without any configurable flag.	
		if err = delEmptyRows(&itemsFound); err != nil {
			numChanges += itemsFound
			return errors.Wrap(err, "ReduceMatrix failed")											
		}

		// Increment counters and print status when done.
		itemsInPass += itemsFound		
		numChanges  += itemsInPass
				
		if itemsInPass == 0 {
			log(pINFO, "Reduction done after %d of %d iterations, %d items removed.\n", 
					i, psControl.MaxIter, numChanges)
			break
		}

	} // End for maximum iterations loop
	
	return nil
}

//==============================================================================

// SolveProb receives a control structure specifying the MPS input file to be read,
// the file where Cplex output should be written (default will be used if not
// specified), the maximum number of iterations Cplex should perform, and boolean
// flags indicating which reduction operations to perform and whether to solve
// the problem.
//
// The function then reads the MPS input file and reduces the problem size by
// iteratively performing the reduction operations specified. If the RunCplex 
// flag is set to false, the function returns at this point.
//
// If the RunCplex flag is set to true, the function then passes the reduced 
// model to Cplex to be solved either as an LP, or as a MIP. It
// then processes the results provided by Cplex and, in conjunction with data
// stored in internal structures about the presolve operations, reconstitues the
// original problem, and returns the result in the psRslt data structure. 
//
// Shadow price and slack values which are typically calculated by Cplex are 
// not calculated for variables and constraints that have been removed during 
// presolving, and hence not passed to Cplex. Those values are set to 0, and the
// status associated with that value is set to "NA" (not available).
//
// In case of failure, function returns an error.
func SolveProb (psc PsCtrl, psRslt *PsSoln) error {
	var numRows            int  // number of rows in the model prior to reduction
	var numCols            int  // number of cols in the model prior to reduction
	var numElem            int  // number of elements in the model prior to reduction
	var coefPerLine        int  // number of coef./line to be printed by WritePsopFile
	var objVal         float64  // value of the objective function returned by Cplex
	var sRows    []gpx.SolnRow  // solved constraints returned by Cplex via gpx
	var sCols    []gpx.SolnCol  // solved variables returned by Cplex via gpx
	var conMap     PsResConMap  // constraint map merged from Cplex and reduction results
	var varMap     PsResVarMap  // variable map  merged from Cplex and reduction results
	var origObjFunc      psRow  // objective function before reductions in post-solve format
	var psRows         []psRow  // original constraints translated to post-solve format
	var err              error  // error returned by secondary functions called
	var colScaleMap  map[string]float64  // map of column scale factors in original model
	

	// Initialize variables.
	psOpList       = nil
	psRslt.ObjVal  = 0
	psRslt.ConMap  = nil
	psRslt.VarMap  = nil
	psRslt.RowsDel = 0
	psRslt.ColsDel = 0
	psRslt.ElemDel = 0
	coefPerLine    = 2

	if psc.FileInMps != "" {
		if err = ReadMpsFile(psc.FileInMps); err != nil {
			return errors.Wrap(err, "SolveProb failed to read file")
		}
		
		// Check that none of the other files have the same name so we don't
		// accidentally overwrite our input file.
		
		if psc.FileInMps == psc.FileOutCplexSoln {
			return errors.Errorf("Cplex solution file cannot overwrite %s", psc.FileInMps)
		}
		
		if psc.FileInMps == psc.FileOutMpsRdcd {
			return errors.Errorf("MPS output file cannot overwrite %s", psc.FileInMps)
		}

		if psc.FileInMps == psc.FileOutPsop {
			return errors.Errorf("PSOP output file cannot overwrite %s", psc.FileInMps)
		}
		
	} // End if populating model from file

	// Record original matrix size.
	numRows = len(Rows)
	numCols = len(Cols)
	numElem = len(Elems)

	// If lists of rows, cols, or elems is empty, return an error.
	
	if numRows <= 0 {
		return errors.Errorf("SolveProb received empty rows list")	
	}
	if numCols <= 0 {
		return errors.Errorf("SolveProb received empty columns list")	
	}
	if numElem <= 0 {
		return errors.Errorf("SolveProb received empty elements list")	
	}

		
	// Translate all original rows to the new format and save the objective function,
	// if it exists, as a separate entity also in the new format.
	
	_ = translateAllRows(&psRows)
	
	if ObjRow >= 0 {
		// If objective function is not the first row in the list, move it there.
		if ObjRow != 0 {
			log(pINFO, "\nMoving %s from index %d to top of list.\n", Rows[ObjRow].Name, ObjRow)
			_ = swapRows(0, ObjRow)
			ObjRow = 0	
		}

		// There is an objective function, save it for later use.
		if err = translateRow(Rows[ObjRow], &origObjFunc); err != nil {
			return errors.Wrap(err, "SolveProb failed")
		}
	}	

	// Make the map which of column scale factors and populate map until the values
	// can be transferred to the Cplex solution.

	colScaleMap = make(map[string]float64)
	
	for i := 0; i < len(Cols); i++ {
		colScaleMap[Cols[i].Name] = Cols[i].ScaleFactor	
	}

	// Remove rows and columns specified in the control structure and calculate
	// how many rows, cols, and elems were removed.			
	if err = ReduceMatrix(psc); err != nil {
		return errors.Wrap(err, "SolveProb failed")
	}
	
	psRslt.RowsDel = numRows - len(Rows)
	psRslt.ColsDel = numCols - len(Cols)
	psRslt.ElemDel = numElem - len(Elems)


	// Write the reduced MPS file if requested.	
	if psc.FileOutMpsRdcd != "" {
		if err = WriteMpsFile(psc.FileOutMpsRdcd); err != nil {
			return errors.Wrap(err, "SolveProb failed")		
		}		
	}

	// Write the Psop file if requested.
	if psc.FileOutPsop != "" {
		if err = WritePsopFile(psc.FileOutPsop, coefPerLine); err != nil {
			return errors.Wrap(err, "SolveProb failed")		
		}				
	}

	// Solution was not requested, so return at this point
	if ! psc.RunCplex {
		return nil		
	}

	// Create the LP using callable C functions.
	if err = CplexCreateProb(); err != nil {
		return errors.Wrap(err, "SolveProb failed")
	}	

	if isMip() {
		// This is a MIP, so use the CPX functions for mixed integer problems.
		if err = gpx.MipOpt(); err != nil {
			return errors.Wrap(err, "SolveProb failed to optimize MIP")				
		}

		if err = gpx.GetMipSolution(&objVal, &sRows, &sCols); err != nil {
			return errors.Wrap(err, "SolveProb failed to get solution")				
		}
				
	} else {
		// This is an LP, so use the CPX functions for LP.
		if err = gpx.LpOpt(); err != nil {
			return errors.Wrap(err, "SolveProb failed to optimize LP")				
		}

		if err = gpx.GetSolution(&objVal, &sRows, &sCols); err != nil {
			return errors.Wrap(err, "SolveProb failed to get solution")				
		}
		
	} // End else this is LP


	// Build the variable and constraint maps, transfer data from original model
	// and merge with results obtained from Cplex.	
	if err = buildVarMap(colScaleMap, sCols, &varMap); err != nil {
		return errors.Wrap(err, "SolveProb failed to process variables")				
	}
	
	_ = buildConMap(sRows, &conMap)

	psRslt.ConMap = conMap
	psRslt.VarMap = varMap

	// Write the Cplex solution to xml file if requested.
	if psc.FileOutCplexSoln != "" {
		if err = gpx.SolWrite(psc.FileOutCplexSoln); err != nil {
			return errors.Wrap(err, "SolveProb failed to write solution to file")		
		}		
	}

	// Close and clean up Cplex.
	if err = gpx.CloseCplex(); err != nil {
		return errors.Wrap(err, "SolveProb failed to close cplex")
	}
	
	// Update the maps with the information deleted during presolve.
	if err = postSolve(psRslt.ConMap, psRslt.VarMap); err != nil {
		return errors.Wrap(err, "SolveProb failed")
	}

	// Restore all other rows that may have been removed and not put back during
	// pre- and postsolve (e.g. non-binding).

	for i := 0; i < len(psRows); i++ {
		_ = addConMapItem(psRslt.ConMap, psRows[i])
	}

	for i := 0; i < len(psRows); i++ {
				
		if mapItem, ok := psRslt.ConMap[psRows[i].Name]; ok {

			mapItem.Dual        = psRslt.ConMap[psRows[i].Name].Dual
			mapItem.Rhs         = psRslt.ConMap[psRows[i].Name].Rhs
			mapItem.Slack       = psRslt.ConMap[psRows[i].Name].Slack 
			mapItem.Status      = psRslt.ConMap[psRows[i].Name].Status
			mapItem.Type        = psRslt.ConMap[psRows[i].Name].Type
			mapItem.ScaleFactor = psRslt.ConMap[psRows[i].Name].ScaleFactor
			psRslt.ConMap[psRows[i].Name] = mapItem
		} else {
			log(pERR, "ERROR: Row %s not found in map.\n", psRows[i].Name)
		}
	}
	 	
	// Calculate the proper value of the objective function.
    // There may have been a constant associated with the objective function 
	// which must now be included in the calculation.
	
	if err = getPstLhs(origObjFunc, psRslt.VarMap, &psRslt.ObjVal); err != nil {
		return errors.Wrap(err, "SolveProb failed")		
	}

	psRslt.ObjVal -= objRowConst
		
	return nil
}

//==============================================================================

// WritePsopFile writes the rows and columns that were removed during the pre-solve
// operations to a text file specified by the user. The function accepts two
// arguments, fileName and coefPerLine. If the file name the file to which the
// output is written, and if empty, a default name is used. 
//
// The coefPerLine specifies how many coefficient name/value pairs should be 
// written per line and is interpretted as follows:
//	  < 0 - all pairs are written on a single line (no CR/LF is inserted between pairs)
//	    0 - printing of coefficient name/value pairs is suppressed
//	    n - a carriage return line feed is inserted after printing n pairs  
// In case of failure, the function returns an error.
func WritePsopFile(fileName string, coefPerLine int) error {

	var opName       string // operation name in more detail than internal var. 
	var rowPresent   bool   // flag indicating that row needs to be printed
	var colPresent   bool   // flag indicating that column needs to be printed
	var index        int    // index tracking how many coefficients were printed
	var printCoef    bool   // controls if coef name/value pairs are printed
	var coefCrNeeded bool   // controls if <CR> printed between coef name/value pairs


	//Check whether the file exists. If it exists, overwrite it.

	if _, err := os.Stat(fileName); err == nil {
		err = os.Remove(fileName)
		if err != nil {
			return errors.Wrapf(err, "Failed to delete existing file %s", fileName)
		}
	}
	
	f, err := os.Create(fileName)
	if err != nil {
		return errors.Wrapf(err, "Failed to create new file %s", fileName)
	}

 	defer f.Close()

	// Set flags to control printing of coefficient name/value pairs per line
	printCoef    = true
	coefCrNeeded = true
	
	if coefPerLine < 0	{
		coefCrNeeded = false
		coefPerLine *= -1
	} else if coefPerLine == 0 {
		printCoef = false
	}

	// Print status message to screen and general header into the file.
	log(pINFO, "\nWriting pre-solve operations to file %s.\n", fileName)

	startTime := time.Now()

	fmt.Fprintf(f, "%s", fileDelim)
	fmt.Fprintf(f, "# LPO record of pre-solve operations\n")	
	fmt.Fprintf(f, "# Problem name: %s\n", Name)
	fmt.Fprintf(f, "# Created on:   %s\n", startTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(f, "#\n# Col format:   COL:  Name  Type  LowerBound  UpperBound  ScaleFactor\n")
	fmt.Fprintf(f, "# Row format:   ROW:  Name  Type  Rhs  ScaleFactor\n")
	
	if printCoef {
		fmt.Fprintf(f, "# Followed by:  CoefName CoefValue (up to %d pairs/line)\n#\n", coefPerLine)
	} else {
		fmt.Fprintf(f, "# Coefficient name/value pairs for rows are not printed.\n#\n")
	}

	// Print the rows and cols associated with each PSOP in the list.
	for i := 0; i < len(psOpList); i++ {

		// Set print flags based on operation type being processed.
		switch psOpList[i].OpType {
			
			case psopEmptyRow:
				opName     = "Empty Row"
				rowPresent = true
				colPresent = false
							
			case psopFixedVar:
				opName     = "Fixed Variable"
				rowPresent = false
				colPresent = true
			
			case psopFreeCol:
				opName     = "Free Column Singleton"
				rowPresent = true
				colPresent = true
			
			case psopNbRow:
				opName     = "Non-binding Row"
				rowPresent = true
				colPresent = false
			
			case psopRowSingltn:
				opName     = "Row Singleton"
				rowPresent = true
				colPresent = true
			
			default:
				opName     = "Unknown Operation"
				rowPresent = false
				colPresent = false
						
		} // End switch on operation type

		// Print the operation type, ID
		fmt.Fprintf(f, "%s", fileDelim)
		fmt.Fprintf(f, "# %s\n", opName)		
		fmt.Fprintf(f, "PSOP: %s %5d\n", psOpList[i].OpType, i)

		if colPresent {
			fmt.Fprintf(f, "COL:  %s   %s %15e %15e %15e\n", 
				psOpList[i].Col.Name, psOpList[i].Col.Type,
				psOpList[i].Col.BndLo, psOpList[i].Col.BndUp, psOpList[i].Col.ScaleFactor)				
		} // End if column was printed		
		
		if rowPresent {
			fmt.Fprintf(f, "ROW:  %s   %s %15e %15e\n", 
				psOpList[i].Row.Name, psOpList[i].Row.Type, 
				psOpList[i].Row.Rhs, psOpList[i].Row.ScaleFactor)

			if printCoef {
				for index = 0; index < len(psOpList[i].Row.Coef); index++ {
					fmt.Fprintf(f, "%15s %15e", psOpList[i].Row.Coef[index].Name, 
							psOpList[i].Row.Coef[index].Value)

					// Print two pairs of coef. per line, and extra CR if odd number
					if coefCrNeeded && ((index + 1) % coefPerLine) == 0 {
						fmt.Fprintf(f, "\n")
					}				
				} // End for list of coefficients

				if coefCrNeeded && index != 0 {
					if ((index) % coefPerLine) != 0 {
						fmt.Fprintf(f, "\n")				
					} // End if extra <CR> needed after last coef.				
				} // End if some coefficients were present			
				
			} // End for printing coefficients
		} // End if row was printed				
	} // End for processing post-solve operations list

	log(pINFO, "Successfully wrote %d operations.\n", len(psOpList))
			
	return nil
}

//==============================================================================
// FUNCTIONS ASSOCIATED WITH CPLEX
//==============================================================================

// cplexInitSoln initializes the data structure used for storing the solution
// obtained by parsing the xml solution file produced by Cplex, and passes the
// initialized structure as the soln argument back to the caller.
// In case of failure, function returns an error.
func cplexInitSoln(soln *CplexSoln) error {
	
	soln.Version                   = ""
	soln.Header.ProblemName        = ""
	soln.Header.ObjValue           = 0.0
	soln.Header.SolTypeValue       = 0
	soln.Header.SolTypeString      = ""
	soln.Header.SolStatusValue     = 0
	soln.Header.SolStatusString    = ""
	soln.Header.SolMethodString    = ""
	soln.Header.PrimalFeasible     = 0
	soln.Header.DualFeasible       = 0
	soln.Header.SimplexItns        = 0
	soln.Header.BarrierItns        = 0
	soln.Header.WriteLevel         = 0
	soln.Quality.EpRHS             = 0.0
	soln.Quality.EpOpt             = 0.0
	soln.Quality.MaxPrimalInfeas   = 0.0
	soln.Quality.MaxDualInfeas     = 0.0
	soln.Quality.MaxPrimalResidual = 0.0
	soln.Quality.MaxDualResidual   = 0.0
	soln.Quality.MaxX              = 0.0
	soln.Quality.MaxPi             = 0.0
	soln.Quality.MaxSlack          = 0.0
	soln.Quality.MaxRedCost        = 0.0
	soln.Quality.Kappa             = 0.0
	soln.LinCons                   = nil
	soln.Varbs                     = nil
	
	return nil
}

//==============================================================================

// CplexParseSoln takes as input the location of the file storing the raw
// output generated by Cplex, parses it, and returns the parsed solution to
// the caller in the soln variable. 
// In case of failure, function returns an error.
func CplexParseSoln(fileName string, soln *CplexSoln) error {
	var err error  // error returned by called functions

	// Initialize the solution data structure.	 
	_ = cplexInitSoln(soln)

	// Open the file containing the Cplex xml output, and defer closing this file.	
	cplexSolnFile, err := os.Open(fileName)
	if err != nil {
		return errors.Wrap(err, "Unable to open cplex solution file")
	}
	defer cplexSolnFile.Close()

	// Parse the xml file and populate the data structure with the results.	
	XMLdata, err := ioutil.ReadAll(cplexSolnFile)
	if err != nil {
		return errors.Wrap(err, "Unable to parse cplex solution file")
	}

	xml.Unmarshal(XMLdata, soln)
	
	return nil
}

//==============================================================================

// CplexSolveMps uses Cplex to solve the problem defined in the MPS file specified.
// The function accepts as input the full path of the MPS file defining the model
// to be processed by Cplex, location of the file to which the solution should
// be written by Cplex, and location of the presolve file to which Cplex writes its
// presolved data. The presolve file is optional and may be omitted (""). 
// The other two files must be specified. 
//
// The function generates a command file, and instructs Cplex to run it. 
// Once complete, the xml output file generated by Cplex is parsed and
// the results are passed back to the caller via the soln variable.
// 
// In case of failure, function returns an error.
//
//	The arguments used by this function are as follows:
//	   mpsFile  [input]: name of MPS file which defines the model
//	   solnFile [input]: name of xml file to which Cplex writes the solution
//	   psFile   [input]: optional name of presolve file, may be empty string
//	   soln    [output]: data structure in which parsed solution is returned  
func CplexSolveMps(mpsFile string, solnFile string, psFile string, soln *CplexSoln) error {

	var bigString    string  // holder for processing stdout text generated by Cplex
	var cplexPsFile  string  // intermediate presolve file used by Cplex
	var cplexCmdFile string  // command file telling Cplex what to do
	var strStart        int  // return value from strings.Index used in parsing stdout  
	var cpTime      float64  // Cplex solution time extracted from stdout
	var err           error  // error returned by secondary functions called

	// Initialize the solution set which may need to be returned if errors occur
	// before any parsing takes place.
	
	_ = cplexInitSoln(soln)
	cplexCmdFile = tempDirPath + "/cpxCommands.txt"
	cplexPsFile  = tempDirPath + "/presolved.pre"
		
	//	var cpxObj float64
	//check whether the output file exists
	if _, err = os.Stat(solnFile); err == nil {
		//if it does exist, remove it
		if err = os.Remove(solnFile); err != nil {
			return errors.Wrap(err, "CplexSolveMps failed to remove solution file")
		}
	}
	//check whether the presolve intermediate file exists
	if _, err := os.Stat(cplexPsFile); err == nil {
		//if it does exist, remove it
		if err = os.Remove(cplexPsFile); err != nil {
			return errors.Wrap(err, "CplexSolveMps failed to remove presolved intermediate file")
		}
	}
	//check whether the presolve output file exists
	if _, err := os.Stat(psFile); err == nil {
		//if it does exist, remove it
		if err = os.Remove(psFile); err != nil {
			return errors.Wrap(err, "CplexSolveMps failed to remove presolve file")
		}
	}

	// Create the command file.
	f, err := os.Create(cplexCmdFile)
	if err != nil {
		return errors.Wrap(err, "CplexSolveMps failed to create command file")
	} 

	fmt.Fprintln(f, "read", mpsFile, "mps")      //command to read the MPS file
	fmt.Fprintln(f, "optimize")                  //optimize command
	fmt.Fprintln(f, "write", solnFile, "sol")    //write the soln file
	
	// TODO: Logic seems convoluted here. See what is intended and clean it up.
	if psFile != "" {
		//include commands to write out the presolved file
		fmt.Fprintln(f, "write", cplexPsFile)
		fmt.Fprintln(f, "read",  cplexPsFile)
		fmt.Fprintln(f, "write", psFile)
	}
	f.Close()      // finish writing the command file
	cmd := "cplex" //start Cplex
	args := []string{"-f", cplexCmdFile}
	out, err := exec.Command(cmd, args...).Output()

	if err != nil {
		return errors.Wrap(err, "Exec command for Cplex failed in CplexSolveMps")
	}

	// Check if this version of cplex can handle the problem
	bigString = string(out)
	if strings.Contains(bigString, "1016: Promotional version") {
		log(pERR, "ERROR: Problem too large for promotional version.")
		return errors.New("Problem too large for promotional version")	
	}

	// TODO: Need a better way to handle errors from cplex. Once we switch to
	// using function calls instead of files, parsing errors from files will not
	// be needed.
	strStart = strings.Index(bigString, "CPLEX Error")
	if strStart >= 0 {
		return errors.New(bigString[strStart:strStart+30])
	}
	
	// Check if some other error not detected above occurred	
	if err != nil {
		return errors.Wrap(err, "CplexSolveMps exec command failed")
	}
	
	// Now parse the solution. The parser initializes the data structure and
	// there is no longer need to initialize global variables.

	err = CplexParseSoln(solnFile, soln)
	if err != nil {
		return errors.Wrap(err, "CplexSolveMps failed")
	}

	// Convert the command window output into a string and parse it
	// to get solution times.
	bigString = string(out)

	// TODO: Find a better way to get the time
	iStart := 0
	for i := iStart; i < len(bigString)-16; i++ {
		if bigString[i:i+15] == "Solution time =" {
			for j := i + 16; j < i+36; j++ {
				if bigString[j:j+4] == "sec." {
					cpTime, _ = strconv.ParseFloat(strings.Trim(bigString[i+16:j-1], " "), 64)
					//log(pINFO, "bigString solution time", bigString[i+16:j-1])
					log(pINFO, "Cplex solution time: %f secs\n", cpTime)
					break
				}
			}
		}
	}
	
	log(pINFO, "Barrier iterations:  %d\n", soln.Header.BarrierItns)
	log(pINFO, "Simplex iterations:  %d\n", soln.Header.SimplexItns)
	
	return nil
}

//==============================================================================

// TransToGpx translates lpo data structures to the gpx data structures which will
// in turn be translated to internal C data structures passed to Cplex. The input
// is taken from the Rows, Cols, and Elems global variables, and the output is
// returned via the variables defined below.
// In case of failure, function returns an error.
//
//	The arguments used by this function are:
//	   gRows [output]: rows, except the objective function, in gpx format
//	   gCols [output]: columns in gpx format
//	   gElem [output]: non-zero elements present in the gRows list
//	   gObj  [output]: non-zero elements present in the objective function
func TransToGpx(gRows *[]gpx.InputRow, gCols *[]gpx.InputCol, gElem *[]gpx.InputElem, 
				gObj *[]gpx.InputObjCoef) error {

	var rowItem   gpx.InputRow     // row item used in constructing gRows list
	var colItem   gpx.InputCol     // col item used in constructing gCols list
	var elemItem  gpx.InputElem    // elem item used in constructing gElem list
	var objItem   gpx.InputObjCoef // item used in constructing coefficients in obj. func.
	var rowIndex  int              // index or row being processed
	var elemIndex int              // index of element being processed

	// Initialize lists that will be returned.
	*gRows = nil
	*gCols = nil
	*gElem = nil
	*gObj  = nil

	// Problem needs to have some rows.
	if len(Rows) == 0 {
		return errors.Errorf("Input list of rows is empty")				
	}

	// Problem needs to have some columns.
	if len(Cols) == 0 {
		return errors.Errorf("Input list of columns is empty")				
	}

	// Problem needs to have some elements.
	if len(Elems) == 0 {
		return errors.Errorf("Input list of elements is empty")				
	}

	// The objective function must be one of the rows.
	if ObjRow < 0 || ObjRow >= len(Rows) {
		return errors.Errorf("Unexpected location of objective function row %d", ObjRow)		
	}

	// Translate the columns data structure
	for i := 0; i < len(Cols); i++ {
		colItem.Name  = Cols[i].Name
		colItem.BndLo = Cols[i].BndLo
		colItem.BndUp = Cols[i].BndUp

		switch Cols[i].Type {

			// At this time, lpo only differentiates between Real and Integer
			// variables. Map them to the values Cplex understands and flag anything
			// else.
			case "R":
				colItem.Type  = "C"
				
			case "I":
				colItem.Type  = "I"

			default:
				return errors.Errorf("Unexpected type %s in col %s", Cols[i].Type, Cols[i].Name)			
			
		}
		
		*gCols = append(*gCols, colItem)		
	}
	
	// Translate the rows data structure. The objective row is handled as a
	// separate entity, because that's how Cplex expects it. We also need to adjust
	// indices to account for one of the rows being the objective function.
	rowIndex = 0
	for i := 0; i < len(Rows); i++ {

		if i == ObjRow {
			// Translate the objective function to its data structure.
			for j := 0; j < len(Rows[i].HasElems); j++ {
				elemIndex = Rows[i].HasElems[j]
				objItem.ColIndex = Elems[elemIndex].InCol
				objItem.Value    = Elems[elemIndex].Value
				*gObj = append(*gObj, objItem)
			} // End for objective function coefficients
		} else {

			rowItem.Name   = Rows[i].Name
			rowItem.Sense  = Rows[i].Type
			rowItem.RngVal = 0.0
			
			switch Rows[i].Type {
				case "L", "E":	
					rowItem.Rhs   = Rows[i].RHSup
				
				case "G":
					rowItem.Rhs   = Rows[i].RHSlo

				case "R":
					rowItem.Rhs   =  Rows[i].RHSlo
					rowItem.RngVal = Rows[i].RHSup - Rows[i].RHSlo

				case "N":
					// In case "N" row type gets past all other safeguards,
					// translate to something that Cplex will not reject.				
					if Rows[i].RHSup != Plinfy {
						rowItem.Sense = "L"
						rowItem.Rhs   = Rows[i].RHSup
					} else if Rows[i].RHSlo != -Plinfy {
						rowItem.Sense = "G"
						rowItem.Rhs   =  Rows[i].RHSlo
					} else {
						return errors.Errorf("Failed to translate non-binding row %s", Rows[i].Name)						
					}
					
							
				default:
					return errors.Errorf("Unexpected type %s in row %s", Rows[i].Type, Rows[i].Name)			
			} // End switch on row type	

			*gRows = append(*gRows, rowItem)
			
			// Translate the non-zero elements. We need to use "rowIndex" to compensate
			// for the first row in the LPO structures being the Objective Function.
			// The column index will match the LPO structures and can be used "as is".
			
			for j := 0; j < len(Rows[i].HasElems); j++ {
				elemIndex           = Rows[i].HasElems[j]
				elemItem.RowIndex   = rowIndex
				elemItem.ColIndex   = Elems[elemIndex].InCol
				elemItem.Value      = Elems[elemIndex].Value
				*gElem = append(*gElem, elemItem)
			}
			
			rowIndex++
								
		} // End else processing rows other than objective function					
	} // End for all rows

	return nil	
}

//==============================================================================

// TransFromGpx translates gpx data structures to the global variables
// Rows, Cols, and Elems in order to create the model.
// In case of failure, function returns an error.
//
//	The arguments used by this function are:
//	   probNm [input]: name of the problem
//	   objNm  [input]: name of the objective function
//	   gRows  [input]: rows, except the objective function, in gpx format
//	   gCols  [input]: columns in gpx format
//	   gElem  [input]: non-zero elements present in the gRows list
//	   gObj   [input]: non-zero elements present in the objective function
func TransFromGpx(probNm string, objNm string, gRows []gpx.InputRow, gCols []gpx.InputCol, 
					gElem []gpx.InputElem, gObj []gpx.InputObjCoef) error {

	var rowItem  InputRow   // temporary holder for row item
	var colItem  InputCol   // temporary holder for column item
	var elemItem InputElem  // temporary holder for element item
	var err      error      // error returned by called functions

	// Check that the received rows, columns, and elements lists are not empty.
	// The other input parameters can be empty and are not checked.
	
	// Problem needs to have some rows.
	if len(gRows) == 0 {
		return errors.Errorf("List of gpx rows is empty")				
	}

	// Problem needs to have some columns.
	if len(gCols) == 0 {
		return errors.Errorf("List of gpx columns is empty")				
	}

	// Problem needs to have some elements.
	if len(gElem) == 0 {
		return errors.Errorf("List of gpx elements is empty")				
	}

	// Initialize the lpo data structures and set the model name.
	
	if err = InitModel(); err != nil {
		return errors.Wrap(err, "Failed to initialize model")
	}	

	// Transfer the problem name passed to function to global variable.
	Name = probNm

	// Translate the columns data structure, and keep same order as in original.
	for i := 0; i < len(gCols); i++ {
		colItem.Name  = gCols[i].Name
		colItem.BndLo = gCols[i].BndLo
		colItem.BndUp = gCols[i].BndUp
		
		switch gCols[i].Type {

			case "C": // Continuous aka real variable
				colItem.Type  = "R"
				
			case "I", "B": // General integer or binary variable
				colItem.Type  = "I"

			case "S": // Semi-continuous variable
				log(pWARN, "WARNING: Only the continuous part of a semi-continuous variable is handled. Lower bound = 1.0.\n")
				colItem.Type  = "I"

			case "N": // TODO: Check if warning needed for semi-integer variable
				colItem.Type  = "I"

			default:
				return errors.Errorf("Unexpected type %s in col %s", Cols[i].Type, Cols[i].Name)			
			
		} // End switch on column type
		
		Cols = append(Cols, colItem)		
	} // End if processing columns


	// Translate the rows data structure, and keep same order as in original.
	for i := 0; i < len(gRows); i++ {
		
		rowItem.Name   = gRows[i].Name
		rowItem.Type   = gRows[i].Sense

		switch gRows[i].Sense {
			case "L":	
				rowItem.RHSlo = -Plinfy
				rowItem.RHSup = gRows[i].Rhs

			case "E":	
				rowItem.RHSlo = gRows[i].Rhs
				rowItem.RHSup = gRows[i].Rhs
				
			case "G":
				rowItem.RHSlo = gRows[i].Rhs
				rowItem.RHSup = Plinfy

			case "R":
				rowItem.RHSlo = gRows[i].Rhs
				rowItem.RHSup = gRows[i].Rhs + gRows[i].RngVal
											
			default:
				return errors.Errorf("Unexpected type %s in row %s", Rows[i].Type, Rows[i].Name)			
			} // End switch on row type	

		Rows = append(Rows, rowItem)
				
	} // End if processing rows
	
	
	// Translate the non-zero elements structure, and keep same order as in original.
	for i := 0; i < len(gElem); i++ {

		elemItem.Value = gElem[i].Value
		elemItem.InRow = gElem[i].RowIndex
		elemItem.InCol = gElem[i].ColIndex

		// Update the Rows and Cols lists to point at the current element
				
		Elems = append(Elems, elemItem)
		Rows[elemItem.InRow].HasElems = append(Rows[elemItem.InRow].HasElems, i)
		Cols[elemItem.InCol].HasElems = append(Cols[elemItem.InCol].HasElems, i)		

	} // End for all non-zero elements
	
	// Process the objective function if it is present, and add it to the end of
	// the rows list.

	if len(gObj) > 0 {
		// If objective function name not received, set it to default.
		if objNm == "" {
			objNm = "ObjFunc"
		}

		rowItem.Name  = objNm
		rowItem.Type  = "N"
		rowItem.RHSlo = 0.0
		rowItem.RHSup = 0.0

		// Build up the elements list for the objective function. Be careful about
		// order of adding items to Elems and Rows lists since the lengths
		// of these lists are used as array indices in the HasElems list of obj. func.
		for i := 0; i < len(gObj); i++ {
			elemItem.Value   = gObj[i].Value
			elemItem.InRow   = len(Rows)
			elemItem.InCol   = gObj[i].ColIndex
			rowItem.HasElems = append(rowItem.HasElems, len(Elems))
			Elems            = append(Elems, elemItem)	
		}

		// Finally add the objective function to the end of the rows list.
		Rows = append(Rows, rowItem)
				
	} // End if objective function is present
			
	
	// Move objective function to top of list and calculate gradient vectors.
	
	if err = AdjustModel(); err != nil {
		return errors.Wrap(err, "Failed to adjust model")		
	}
	
	return nil									
}


//==============================================================================

// CplexCreateProb initializes the Cplex environment, translates the model from
// the global Rows, Cols, and Elems variables to data structures used by the gpx
// package, and uses gpx to build the model in Cplex so that it may be solved by
// separate function calls.
// In case of failure, function returns an error.
func CplexCreateProb() error {
	var gRows []gpx.InputRow     // rows data structure, excluding the objective function
	var gCols []gpx.InputCol     // cols data structure
	var gElem []gpx.InputElem    // non-zero elements present in the rows structure
	var gObj  []gpx.InputObjCoef // non-zero elements in the objective function
	var err     error            // error returned by secondary functions called

	// Initialize the Cplex environment and assign the problem name based on the
	// lpo global problem "Name".	
	if err = gpx.CreateProb(Name); err != nil {
		return errors.Wrap(err, "CplexCreateProb failed to create problem")
	}

	// If logLevel is high enough, have output printed to screen.
	if logLevel >= pINFO {
		if err = gpx.OutputToScreen(true); err != nil {
			return errors.Wrap(err, "CplexCreateProb ailed to set output to screen")
		}
	}

	// Translate from lpo global data structures to the gpx data structures.	
	err = TransToGpx(&gRows, &gCols, &gElem, &gObj)
	if err != nil {
		return errors.Wrap(err, "CplexCreateProb failed to translate to gpx data structures")
	}

	// Populate the rows of the problem.
	if err = gpx.NewRows(gRows); err != nil {
		return errors.Wrap(err, "CplexCreateProb failed to create rows")		
	}			

	// Populate the columns and objective function of the problem.
	if err = gpx.NewCols(gObj, gCols); err != nil {
		return errors.Wrap(err, "CplexCreateProb failed to create columns")		
	}			

	// Change the coefficients of the problem to their non-zero values.
	if err = gpx.ChgCoefList(gElem); err != nil {
		return errors.Wrap(err, "CplexCreateProb failed to create elements")		
	}			

	return nil			
}


