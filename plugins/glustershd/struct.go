package glustershd

import (

        "encoding/xml"
)

type Brick struct {
                HostId                          string  `xml:"hostUuid,attr"json:"hostId"`
                Name                            string  `xml:"name"json:"name"`
                Status                          string  `xml:"status"json:"status"`
                TotalNumberOfEntries            int     `xml:"totalNumberOfEntries"json:"totalNumberOfEntries",omitempty`
                NumberOfEntriesInHealPending    int     `xml:"numberOfEntriesInHealPending"json:"numberOfEntriesInHealPending",omitempty`
                NumberOfEntriesInSplitBrain     int     `xml:"numberOfEntriesInSplitBrain"json:"numberOfEntriesInSplitBrain",omitempty`
                NumberOfEntriesPossiblyHealing  int     `xml:"numberOfEntriesPossiblyHealing"json:"numberOfEntriesPossiblyHealing",omitempty`
                NumberOfEntries                 int     `xml:"numberOfEntries"json:"numberOfEntries",omitempty`
}

type HealInfo struct {
        XMLNAME         xml.Name        `xml:"cliOutput"`
        Bricks          []Brick         `xml:"healInfo>bricks>brick"`
}
