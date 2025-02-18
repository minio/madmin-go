package madmin

// Code generated by github.com/tinylib/msgp DO NOT EDIT.

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *ClusterInfo) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "version":
			z.Version, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Version")
				return
			}
		case "deploymentID":
			z.DeploymentID, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "DeploymentID")
				return
			}
		case "siteName":
			z.SiteName, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "SiteName")
				return
			}
		case "siteRegion":
			z.SiteRegion, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "SiteRegion")
				return
			}
		case "license":
			var zb0002 uint32
			zb0002, err = dc.ReadMapHeader()
			if err != nil {
				err = msgp.WrapError(err, "License")
				return
			}
			for zb0002 > 0 {
				zb0002--
				field, err = dc.ReadMapKeyPtr()
				if err != nil {
					err = msgp.WrapError(err, "License")
					return
				}
				switch msgp.UnsafeString(field) {
				case "org":
					z.License.Organization, err = dc.ReadString()
					if err != nil {
						err = msgp.WrapError(err, "License", "Organization")
						return
					}
				case "type":
					z.License.Type, err = dc.ReadString()
					if err != nil {
						err = msgp.WrapError(err, "License", "Type")
						return
					}
				case "expiry":
					z.License.Expiry, err = dc.ReadString()
					if err != nil {
						err = msgp.WrapError(err, "License", "Expiry")
						return
					}
				default:
					err = dc.Skip()
					if err != nil {
						err = msgp.WrapError(err, "License")
						return
					}
				}
			}
		case "platform":
			z.Platform, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Platform")
				return
			}
		case "domain":
			var zb0003 uint32
			zb0003, err = dc.ReadArrayHeader()
			if err != nil {
				err = msgp.WrapError(err, "Domain")
				return
			}
			if cap(z.Domain) >= int(zb0003) {
				z.Domain = (z.Domain)[:zb0003]
			} else {
				z.Domain = make([]string, zb0003)
			}
			for za0001 := range z.Domain {
				z.Domain[za0001], err = dc.ReadString()
				if err != nil {
					err = msgp.WrapError(err, "Domain", za0001)
					return
				}
			}
		case "pools":
			var zb0004 uint32
			zb0004, err = dc.ReadArrayHeader()
			if err != nil {
				err = msgp.WrapError(err, "Pools")
				return
			}
			if cap(z.Pools) >= int(zb0004) {
				z.Pools = (z.Pools)[:zb0004]
			} else {
				z.Pools = make([]PoolInfo, zb0004)
			}
			for za0002 := range z.Pools {
				err = z.Pools[za0002].DecodeMsg(dc)
				if err != nil {
					err = msgp.WrapError(err, "Pools", za0002)
					return
				}
			}
		case "metrics":
			var zb0005 uint32
			zb0005, err = dc.ReadMapHeader()
			if err != nil {
				err = msgp.WrapError(err, "Metrics")
				return
			}
			for zb0005 > 0 {
				zb0005--
				field, err = dc.ReadMapKeyPtr()
				if err != nil {
					err = msgp.WrapError(err, "Metrics")
					return
				}
				switch msgp.UnsafeString(field) {
				case "buckets":
					z.Metrics.Buckets, err = dc.ReadInt()
					if err != nil {
						err = msgp.WrapError(err, "Metrics", "Buckets")
						return
					}
				case "objects":
					z.Metrics.Objects, err = dc.ReadInt()
					if err != nil {
						err = msgp.WrapError(err, "Metrics", "Objects")
						return
					}
				case "versions":
					z.Metrics.Versions, err = dc.ReadInt()
					if err != nil {
						err = msgp.WrapError(err, "Metrics", "Versions")
						return
					}
				case "deleteMarkers":
					z.Metrics.DeleteMarkers, err = dc.ReadInt()
					if err != nil {
						err = msgp.WrapError(err, "Metrics", "DeleteMarkers")
						return
					}
				case "usage":
					z.Metrics.Usage, err = dc.ReadInt64()
					if err != nil {
						err = msgp.WrapError(err, "Metrics", "Usage")
						return
					}
				default:
					err = dc.Skip()
					if err != nil {
						err = msgp.WrapError(err, "Metrics")
						return
					}
				}
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *ClusterInfo) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 9
	// write "version"
	err = en.Append(0x89, 0xa7, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e)
	if err != nil {
		return
	}
	err = en.WriteString(z.Version)
	if err != nil {
		err = msgp.WrapError(err, "Version")
		return
	}
	// write "deploymentID"
	err = en.Append(0xac, 0x64, 0x65, 0x70, 0x6c, 0x6f, 0x79, 0x6d, 0x65, 0x6e, 0x74, 0x49, 0x44)
	if err != nil {
		return
	}
	err = en.WriteString(z.DeploymentID)
	if err != nil {
		err = msgp.WrapError(err, "DeploymentID")
		return
	}
	// write "siteName"
	err = en.Append(0xa8, 0x73, 0x69, 0x74, 0x65, 0x4e, 0x61, 0x6d, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.SiteName)
	if err != nil {
		err = msgp.WrapError(err, "SiteName")
		return
	}
	// write "siteRegion"
	err = en.Append(0xaa, 0x73, 0x69, 0x74, 0x65, 0x52, 0x65, 0x67, 0x69, 0x6f, 0x6e)
	if err != nil {
		return
	}
	err = en.WriteString(z.SiteRegion)
	if err != nil {
		err = msgp.WrapError(err, "SiteRegion")
		return
	}
	// write "license"
	err = en.Append(0xa7, 0x6c, 0x69, 0x63, 0x65, 0x6e, 0x73, 0x65)
	if err != nil {
		return
	}
	// map header, size 3
	// write "org"
	err = en.Append(0x83, 0xa3, 0x6f, 0x72, 0x67)
	if err != nil {
		return
	}
	err = en.WriteString(z.License.Organization)
	if err != nil {
		err = msgp.WrapError(err, "License", "Organization")
		return
	}
	// write "type"
	err = en.Append(0xa4, 0x74, 0x79, 0x70, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.License.Type)
	if err != nil {
		err = msgp.WrapError(err, "License", "Type")
		return
	}
	// write "expiry"
	err = en.Append(0xa6, 0x65, 0x78, 0x70, 0x69, 0x72, 0x79)
	if err != nil {
		return
	}
	err = en.WriteString(z.License.Expiry)
	if err != nil {
		err = msgp.WrapError(err, "License", "Expiry")
		return
	}
	// write "platform"
	err = en.Append(0xa8, 0x70, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d)
	if err != nil {
		return
	}
	err = en.WriteString(z.Platform)
	if err != nil {
		err = msgp.WrapError(err, "Platform")
		return
	}
	// write "domain"
	err = en.Append(0xa6, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Domain)))
	if err != nil {
		err = msgp.WrapError(err, "Domain")
		return
	}
	for za0001 := range z.Domain {
		err = en.WriteString(z.Domain[za0001])
		if err != nil {
			err = msgp.WrapError(err, "Domain", za0001)
			return
		}
	}
	// write "pools"
	err = en.Append(0xa5, 0x70, 0x6f, 0x6f, 0x6c, 0x73)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Pools)))
	if err != nil {
		err = msgp.WrapError(err, "Pools")
		return
	}
	for za0002 := range z.Pools {
		err = z.Pools[za0002].EncodeMsg(en)
		if err != nil {
			err = msgp.WrapError(err, "Pools", za0002)
			return
		}
	}
	// write "metrics"
	err = en.Append(0xa7, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73)
	if err != nil {
		return
	}
	// map header, size 5
	// write "buckets"
	err = en.Append(0x85, 0xa7, 0x62, 0x75, 0x63, 0x6b, 0x65, 0x74, 0x73)
	if err != nil {
		return
	}
	err = en.WriteInt(z.Metrics.Buckets)
	if err != nil {
		err = msgp.WrapError(err, "Metrics", "Buckets")
		return
	}
	// write "objects"
	err = en.Append(0xa7, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x73)
	if err != nil {
		return
	}
	err = en.WriteInt(z.Metrics.Objects)
	if err != nil {
		err = msgp.WrapError(err, "Metrics", "Objects")
		return
	}
	// write "versions"
	err = en.Append(0xa8, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x73)
	if err != nil {
		return
	}
	err = en.WriteInt(z.Metrics.Versions)
	if err != nil {
		err = msgp.WrapError(err, "Metrics", "Versions")
		return
	}
	// write "deleteMarkers"
	err = en.Append(0xad, 0x64, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4d, 0x61, 0x72, 0x6b, 0x65, 0x72, 0x73)
	if err != nil {
		return
	}
	err = en.WriteInt(z.Metrics.DeleteMarkers)
	if err != nil {
		err = msgp.WrapError(err, "Metrics", "DeleteMarkers")
		return
	}
	// write "usage"
	err = en.Append(0xa5, 0x75, 0x73, 0x61, 0x67, 0x65)
	if err != nil {
		return
	}
	err = en.WriteInt64(z.Metrics.Usage)
	if err != nil {
		err = msgp.WrapError(err, "Metrics", "Usage")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *ClusterInfo) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 9
	// string "version"
	o = append(o, 0x89, 0xa7, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e)
	o = msgp.AppendString(o, z.Version)
	// string "deploymentID"
	o = append(o, 0xac, 0x64, 0x65, 0x70, 0x6c, 0x6f, 0x79, 0x6d, 0x65, 0x6e, 0x74, 0x49, 0x44)
	o = msgp.AppendString(o, z.DeploymentID)
	// string "siteName"
	o = append(o, 0xa8, 0x73, 0x69, 0x74, 0x65, 0x4e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.SiteName)
	// string "siteRegion"
	o = append(o, 0xaa, 0x73, 0x69, 0x74, 0x65, 0x52, 0x65, 0x67, 0x69, 0x6f, 0x6e)
	o = msgp.AppendString(o, z.SiteRegion)
	// string "license"
	o = append(o, 0xa7, 0x6c, 0x69, 0x63, 0x65, 0x6e, 0x73, 0x65)
	// map header, size 3
	// string "org"
	o = append(o, 0x83, 0xa3, 0x6f, 0x72, 0x67)
	o = msgp.AppendString(o, z.License.Organization)
	// string "type"
	o = append(o, 0xa4, 0x74, 0x79, 0x70, 0x65)
	o = msgp.AppendString(o, z.License.Type)
	// string "expiry"
	o = append(o, 0xa6, 0x65, 0x78, 0x70, 0x69, 0x72, 0x79)
	o = msgp.AppendString(o, z.License.Expiry)
	// string "platform"
	o = append(o, 0xa8, 0x70, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d)
	o = msgp.AppendString(o, z.Platform)
	// string "domain"
	o = append(o, 0xa6, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Domain)))
	for za0001 := range z.Domain {
		o = msgp.AppendString(o, z.Domain[za0001])
	}
	// string "pools"
	o = append(o, 0xa5, 0x70, 0x6f, 0x6f, 0x6c, 0x73)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Pools)))
	for za0002 := range z.Pools {
		o, err = z.Pools[za0002].MarshalMsg(o)
		if err != nil {
			err = msgp.WrapError(err, "Pools", za0002)
			return
		}
	}
	// string "metrics"
	o = append(o, 0xa7, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73)
	// map header, size 5
	// string "buckets"
	o = append(o, 0x85, 0xa7, 0x62, 0x75, 0x63, 0x6b, 0x65, 0x74, 0x73)
	o = msgp.AppendInt(o, z.Metrics.Buckets)
	// string "objects"
	o = append(o, 0xa7, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x73)
	o = msgp.AppendInt(o, z.Metrics.Objects)
	// string "versions"
	o = append(o, 0xa8, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x73)
	o = msgp.AppendInt(o, z.Metrics.Versions)
	// string "deleteMarkers"
	o = append(o, 0xad, 0x64, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4d, 0x61, 0x72, 0x6b, 0x65, 0x72, 0x73)
	o = msgp.AppendInt(o, z.Metrics.DeleteMarkers)
	// string "usage"
	o = append(o, 0xa5, 0x75, 0x73, 0x61, 0x67, 0x65)
	o = msgp.AppendInt64(o, z.Metrics.Usage)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ClusterInfo) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "version":
			z.Version, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Version")
				return
			}
		case "deploymentID":
			z.DeploymentID, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "DeploymentID")
				return
			}
		case "siteName":
			z.SiteName, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "SiteName")
				return
			}
		case "siteRegion":
			z.SiteRegion, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "SiteRegion")
				return
			}
		case "license":
			var zb0002 uint32
			zb0002, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "License")
				return
			}
			for zb0002 > 0 {
				zb0002--
				field, bts, err = msgp.ReadMapKeyZC(bts)
				if err != nil {
					err = msgp.WrapError(err, "License")
					return
				}
				switch msgp.UnsafeString(field) {
				case "org":
					z.License.Organization, bts, err = msgp.ReadStringBytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "License", "Organization")
						return
					}
				case "type":
					z.License.Type, bts, err = msgp.ReadStringBytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "License", "Type")
						return
					}
				case "expiry":
					z.License.Expiry, bts, err = msgp.ReadStringBytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "License", "Expiry")
						return
					}
				default:
					bts, err = msgp.Skip(bts)
					if err != nil {
						err = msgp.WrapError(err, "License")
						return
					}
				}
			}
		case "platform":
			z.Platform, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Platform")
				return
			}
		case "domain":
			var zb0003 uint32
			zb0003, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Domain")
				return
			}
			if cap(z.Domain) >= int(zb0003) {
				z.Domain = (z.Domain)[:zb0003]
			} else {
				z.Domain = make([]string, zb0003)
			}
			for za0001 := range z.Domain {
				z.Domain[za0001], bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "Domain", za0001)
					return
				}
			}
		case "pools":
			var zb0004 uint32
			zb0004, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Pools")
				return
			}
			if cap(z.Pools) >= int(zb0004) {
				z.Pools = (z.Pools)[:zb0004]
			} else {
				z.Pools = make([]PoolInfo, zb0004)
			}
			for za0002 := range z.Pools {
				bts, err = z.Pools[za0002].UnmarshalMsg(bts)
				if err != nil {
					err = msgp.WrapError(err, "Pools", za0002)
					return
				}
			}
		case "metrics":
			var zb0005 uint32
			zb0005, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Metrics")
				return
			}
			for zb0005 > 0 {
				zb0005--
				field, bts, err = msgp.ReadMapKeyZC(bts)
				if err != nil {
					err = msgp.WrapError(err, "Metrics")
					return
				}
				switch msgp.UnsafeString(field) {
				case "buckets":
					z.Metrics.Buckets, bts, err = msgp.ReadIntBytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "Metrics", "Buckets")
						return
					}
				case "objects":
					z.Metrics.Objects, bts, err = msgp.ReadIntBytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "Metrics", "Objects")
						return
					}
				case "versions":
					z.Metrics.Versions, bts, err = msgp.ReadIntBytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "Metrics", "Versions")
						return
					}
				case "deleteMarkers":
					z.Metrics.DeleteMarkers, bts, err = msgp.ReadIntBytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "Metrics", "DeleteMarkers")
						return
					}
				case "usage":
					z.Metrics.Usage, bts, err = msgp.ReadInt64Bytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "Metrics", "Usage")
						return
					}
				default:
					bts, err = msgp.Skip(bts)
					if err != nil {
						err = msgp.WrapError(err, "Metrics")
						return
					}
				}
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *ClusterInfo) Msgsize() (s int) {
	s = 1 + 8 + msgp.StringPrefixSize + len(z.Version) + 13 + msgp.StringPrefixSize + len(z.DeploymentID) + 9 + msgp.StringPrefixSize + len(z.SiteName) + 11 + msgp.StringPrefixSize + len(z.SiteRegion) + 8 + 1 + 4 + msgp.StringPrefixSize + len(z.License.Organization) + 5 + msgp.StringPrefixSize + len(z.License.Type) + 7 + msgp.StringPrefixSize + len(z.License.Expiry) + 9 + msgp.StringPrefixSize + len(z.Platform) + 7 + msgp.ArrayHeaderSize
	for za0001 := range z.Domain {
		s += msgp.StringPrefixSize + len(z.Domain[za0001])
	}
	s += 6 + msgp.ArrayHeaderSize
	for za0002 := range z.Pools {
		s += z.Pools[za0002].Msgsize()
	}
	s += 8 + 1 + 8 + msgp.IntSize + 8 + msgp.IntSize + 9 + msgp.IntSize + 14 + msgp.IntSize + 6 + msgp.Int64Size
	return
}

// DecodeMsg implements msgp.Decodable
func (z *PoolInfo) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	var zb0001Mask uint8 /* 1 bits */
	_ = zb0001Mask
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "index":
			z.Index, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "Index")
				return
			}
		case "nodes":
			var zb0002 uint32
			zb0002, err = dc.ReadMapHeader()
			if err != nil {
				err = msgp.WrapError(err, "Nodes")
				return
			}
			for zb0002 > 0 {
				zb0002--
				field, err = dc.ReadMapKeyPtr()
				if err != nil {
					err = msgp.WrapError(err, "Nodes")
					return
				}
				switch msgp.UnsafeString(field) {
				case "total":
					z.Nodes.Total, err = dc.ReadInt()
					if err != nil {
						err = msgp.WrapError(err, "Nodes", "Total")
						return
					}
				case "offline":
					z.Nodes.Offline, err = dc.ReadInt()
					if err != nil {
						err = msgp.WrapError(err, "Nodes", "Offline")
						return
					}
				default:
					err = dc.Skip()
					if err != nil {
						err = msgp.WrapError(err, "Nodes")
						return
					}
				}
			}
		case "Drives":
			var zb0003 uint32
			zb0003, err = dc.ReadMapHeader()
			if err != nil {
				err = msgp.WrapError(err, "Drives")
				return
			}
			for zb0003 > 0 {
				zb0003--
				field, err = dc.ReadMapKeyPtr()
				if err != nil {
					err = msgp.WrapError(err, "Drives")
					return
				}
				switch msgp.UnsafeString(field) {
				case "perNode":
					z.Drives.PerNodeTotal, err = dc.ReadInt()
					if err != nil {
						err = msgp.WrapError(err, "Drives", "PerNodeTotal")
						return
					}
				case "perNodeOffline":
					z.Drives.PerNodeOffline, err = dc.ReadInt()
					if err != nil {
						err = msgp.WrapError(err, "Drives", "PerNodeOffline")
						return
					}
				default:
					err = dc.Skip()
					if err != nil {
						err = msgp.WrapError(err, "Drives")
						return
					}
				}
			}
		case "numberOfSets":
			z.TotalSets, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "TotalSets")
				return
			}
		case "stripeSize":
			z.StripeSize, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "StripeSize")
				return
			}
		case "writeQuorum":
			z.WriteQuorum, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "WriteQuorum")
				return
			}
		case "readQuorum":
			z.ReadQuorum, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "ReadQuorum")
				return
			}
		case "hosts":
			var zb0004 uint32
			zb0004, err = dc.ReadArrayHeader()
			if err != nil {
				err = msgp.WrapError(err, "Hosts")
				return
			}
			if cap(z.Hosts) >= int(zb0004) {
				z.Hosts = (z.Hosts)[:zb0004]
			} else {
				z.Hosts = make([]string, zb0004)
			}
			for za0001 := range z.Hosts {
				z.Hosts[za0001], err = dc.ReadString()
				if err != nil {
					err = msgp.WrapError(err, "Hosts", za0001)
					return
				}
			}
			zb0001Mask |= 0x1
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	// Clear omitted fields.
	if (zb0001Mask & 0x1) == 0 {
		z.Hosts = nil
	}

	return
}

// EncodeMsg implements msgp.Encodable
func (z *PoolInfo) EncodeMsg(en *msgp.Writer) (err error) {
	// check for omitted fields
	zb0001Len := uint32(8)
	var zb0001Mask uint8 /* 8 bits */
	_ = zb0001Mask
	if z.Hosts == nil {
		zb0001Len--
		zb0001Mask |= 0x80
	}
	// variable map header, size zb0001Len
	err = en.Append(0x80 | uint8(zb0001Len))
	if err != nil {
		return
	}

	// skip if no fields are to be emitted
	if zb0001Len != 0 {
		// write "index"
		err = en.Append(0xa5, 0x69, 0x6e, 0x64, 0x65, 0x78)
		if err != nil {
			return
		}
		err = en.WriteInt(z.Index)
		if err != nil {
			err = msgp.WrapError(err, "Index")
			return
		}
		// write "nodes"
		err = en.Append(0xa5, 0x6e, 0x6f, 0x64, 0x65, 0x73)
		if err != nil {
			return
		}
		// map header, size 2
		// write "total"
		err = en.Append(0x82, 0xa5, 0x74, 0x6f, 0x74, 0x61, 0x6c)
		if err != nil {
			return
		}
		err = en.WriteInt(z.Nodes.Total)
		if err != nil {
			err = msgp.WrapError(err, "Nodes", "Total")
			return
		}
		// write "offline"
		err = en.Append(0xa7, 0x6f, 0x66, 0x66, 0x6c, 0x69, 0x6e, 0x65)
		if err != nil {
			return
		}
		err = en.WriteInt(z.Nodes.Offline)
		if err != nil {
			err = msgp.WrapError(err, "Nodes", "Offline")
			return
		}
		// write "Drives"
		err = en.Append(0xa6, 0x44, 0x72, 0x69, 0x76, 0x65, 0x73)
		if err != nil {
			return
		}
		// map header, size 2
		// write "perNode"
		err = en.Append(0x82, 0xa7, 0x70, 0x65, 0x72, 0x4e, 0x6f, 0x64, 0x65)
		if err != nil {
			return
		}
		err = en.WriteInt(z.Drives.PerNodeTotal)
		if err != nil {
			err = msgp.WrapError(err, "Drives", "PerNodeTotal")
			return
		}
		// write "perNodeOffline"
		err = en.Append(0xae, 0x70, 0x65, 0x72, 0x4e, 0x6f, 0x64, 0x65, 0x4f, 0x66, 0x66, 0x6c, 0x69, 0x6e, 0x65)
		if err != nil {
			return
		}
		err = en.WriteInt(z.Drives.PerNodeOffline)
		if err != nil {
			err = msgp.WrapError(err, "Drives", "PerNodeOffline")
			return
		}
		// write "numberOfSets"
		err = en.Append(0xac, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x4f, 0x66, 0x53, 0x65, 0x74, 0x73)
		if err != nil {
			return
		}
		err = en.WriteInt(z.TotalSets)
		if err != nil {
			err = msgp.WrapError(err, "TotalSets")
			return
		}
		// write "stripeSize"
		err = en.Append(0xaa, 0x73, 0x74, 0x72, 0x69, 0x70, 0x65, 0x53, 0x69, 0x7a, 0x65)
		if err != nil {
			return
		}
		err = en.WriteInt(z.StripeSize)
		if err != nil {
			err = msgp.WrapError(err, "StripeSize")
			return
		}
		// write "writeQuorum"
		err = en.Append(0xab, 0x77, 0x72, 0x69, 0x74, 0x65, 0x51, 0x75, 0x6f, 0x72, 0x75, 0x6d)
		if err != nil {
			return
		}
		err = en.WriteInt(z.WriteQuorum)
		if err != nil {
			err = msgp.WrapError(err, "WriteQuorum")
			return
		}
		// write "readQuorum"
		err = en.Append(0xaa, 0x72, 0x65, 0x61, 0x64, 0x51, 0x75, 0x6f, 0x72, 0x75, 0x6d)
		if err != nil {
			return
		}
		err = en.WriteInt(z.ReadQuorum)
		if err != nil {
			err = msgp.WrapError(err, "ReadQuorum")
			return
		}
		if (zb0001Mask & 0x80) == 0 { // if not omitted
			// write "hosts"
			err = en.Append(0xa5, 0x68, 0x6f, 0x73, 0x74, 0x73)
			if err != nil {
				return
			}
			err = en.WriteArrayHeader(uint32(len(z.Hosts)))
			if err != nil {
				err = msgp.WrapError(err, "Hosts")
				return
			}
			for za0001 := range z.Hosts {
				err = en.WriteString(z.Hosts[za0001])
				if err != nil {
					err = msgp.WrapError(err, "Hosts", za0001)
					return
				}
			}
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *PoolInfo) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// check for omitted fields
	zb0001Len := uint32(8)
	var zb0001Mask uint8 /* 8 bits */
	_ = zb0001Mask
	if z.Hosts == nil {
		zb0001Len--
		zb0001Mask |= 0x80
	}
	// variable map header, size zb0001Len
	o = append(o, 0x80|uint8(zb0001Len))

	// skip if no fields are to be emitted
	if zb0001Len != 0 {
		// string "index"
		o = append(o, 0xa5, 0x69, 0x6e, 0x64, 0x65, 0x78)
		o = msgp.AppendInt(o, z.Index)
		// string "nodes"
		o = append(o, 0xa5, 0x6e, 0x6f, 0x64, 0x65, 0x73)
		// map header, size 2
		// string "total"
		o = append(o, 0x82, 0xa5, 0x74, 0x6f, 0x74, 0x61, 0x6c)
		o = msgp.AppendInt(o, z.Nodes.Total)
		// string "offline"
		o = append(o, 0xa7, 0x6f, 0x66, 0x66, 0x6c, 0x69, 0x6e, 0x65)
		o = msgp.AppendInt(o, z.Nodes.Offline)
		// string "Drives"
		o = append(o, 0xa6, 0x44, 0x72, 0x69, 0x76, 0x65, 0x73)
		// map header, size 2
		// string "perNode"
		o = append(o, 0x82, 0xa7, 0x70, 0x65, 0x72, 0x4e, 0x6f, 0x64, 0x65)
		o = msgp.AppendInt(o, z.Drives.PerNodeTotal)
		// string "perNodeOffline"
		o = append(o, 0xae, 0x70, 0x65, 0x72, 0x4e, 0x6f, 0x64, 0x65, 0x4f, 0x66, 0x66, 0x6c, 0x69, 0x6e, 0x65)
		o = msgp.AppendInt(o, z.Drives.PerNodeOffline)
		// string "numberOfSets"
		o = append(o, 0xac, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x4f, 0x66, 0x53, 0x65, 0x74, 0x73)
		o = msgp.AppendInt(o, z.TotalSets)
		// string "stripeSize"
		o = append(o, 0xaa, 0x73, 0x74, 0x72, 0x69, 0x70, 0x65, 0x53, 0x69, 0x7a, 0x65)
		o = msgp.AppendInt(o, z.StripeSize)
		// string "writeQuorum"
		o = append(o, 0xab, 0x77, 0x72, 0x69, 0x74, 0x65, 0x51, 0x75, 0x6f, 0x72, 0x75, 0x6d)
		o = msgp.AppendInt(o, z.WriteQuorum)
		// string "readQuorum"
		o = append(o, 0xaa, 0x72, 0x65, 0x61, 0x64, 0x51, 0x75, 0x6f, 0x72, 0x75, 0x6d)
		o = msgp.AppendInt(o, z.ReadQuorum)
		if (zb0001Mask & 0x80) == 0 { // if not omitted
			// string "hosts"
			o = append(o, 0xa5, 0x68, 0x6f, 0x73, 0x74, 0x73)
			o = msgp.AppendArrayHeader(o, uint32(len(z.Hosts)))
			for za0001 := range z.Hosts {
				o = msgp.AppendString(o, z.Hosts[za0001])
			}
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *PoolInfo) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	var zb0001Mask uint8 /* 1 bits */
	_ = zb0001Mask
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "index":
			z.Index, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Index")
				return
			}
		case "nodes":
			var zb0002 uint32
			zb0002, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Nodes")
				return
			}
			for zb0002 > 0 {
				zb0002--
				field, bts, err = msgp.ReadMapKeyZC(bts)
				if err != nil {
					err = msgp.WrapError(err, "Nodes")
					return
				}
				switch msgp.UnsafeString(field) {
				case "total":
					z.Nodes.Total, bts, err = msgp.ReadIntBytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "Nodes", "Total")
						return
					}
				case "offline":
					z.Nodes.Offline, bts, err = msgp.ReadIntBytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "Nodes", "Offline")
						return
					}
				default:
					bts, err = msgp.Skip(bts)
					if err != nil {
						err = msgp.WrapError(err, "Nodes")
						return
					}
				}
			}
		case "Drives":
			var zb0003 uint32
			zb0003, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Drives")
				return
			}
			for zb0003 > 0 {
				zb0003--
				field, bts, err = msgp.ReadMapKeyZC(bts)
				if err != nil {
					err = msgp.WrapError(err, "Drives")
					return
				}
				switch msgp.UnsafeString(field) {
				case "perNode":
					z.Drives.PerNodeTotal, bts, err = msgp.ReadIntBytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "Drives", "PerNodeTotal")
						return
					}
				case "perNodeOffline":
					z.Drives.PerNodeOffline, bts, err = msgp.ReadIntBytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "Drives", "PerNodeOffline")
						return
					}
				default:
					bts, err = msgp.Skip(bts)
					if err != nil {
						err = msgp.WrapError(err, "Drives")
						return
					}
				}
			}
		case "numberOfSets":
			z.TotalSets, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "TotalSets")
				return
			}
		case "stripeSize":
			z.StripeSize, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "StripeSize")
				return
			}
		case "writeQuorum":
			z.WriteQuorum, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "WriteQuorum")
				return
			}
		case "readQuorum":
			z.ReadQuorum, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "ReadQuorum")
				return
			}
		case "hosts":
			var zb0004 uint32
			zb0004, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Hosts")
				return
			}
			if cap(z.Hosts) >= int(zb0004) {
				z.Hosts = (z.Hosts)[:zb0004]
			} else {
				z.Hosts = make([]string, zb0004)
			}
			for za0001 := range z.Hosts {
				z.Hosts[za0001], bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "Hosts", za0001)
					return
				}
			}
			zb0001Mask |= 0x1
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	// Clear omitted fields.
	if (zb0001Mask & 0x1) == 0 {
		z.Hosts = nil
	}

	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *PoolInfo) Msgsize() (s int) {
	s = 1 + 6 + msgp.IntSize + 6 + 1 + 6 + msgp.IntSize + 8 + msgp.IntSize + 7 + 1 + 8 + msgp.IntSize + 15 + msgp.IntSize + 13 + msgp.IntSize + 11 + msgp.IntSize + 12 + msgp.IntSize + 11 + msgp.IntSize + 6 + msgp.ArrayHeaderSize
	for za0001 := range z.Hosts {
		s += msgp.StringPrefixSize + len(z.Hosts[za0001])
	}
	return
}
